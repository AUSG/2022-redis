# Using Redis

## Client-side caching
네트워크 I/O가 비싸니까 로컬 메모리에 데이터를 저장해두고 쓰자. 
다수의 인스턴스를 운영하고 단일 DB를 운영한다고 했을 때, 모든 인스턴스에 live data 를 서빙해야한다.

### The Redis implementation of client-side Caching
`CLIENT TRACKING` 을 사용하면 된다. TRACKING 은 두 가지 모드가 있는다.

#### default mode
서버가 접근한 클라이언트를 기억하고 있다.  
그리고 키가 변경되었을 때 저장되어 있는 client들에게 invalidation message를 전송한다. 

default mode의 sequence는 다음과 같다.
<img width="342" alt="스크린샷 2022-12-10 오전 11 52 25" src="https://user-images.githubusercontent.com/34934883/206825241-a4997138-a919-46fc-8b82-554edc37c957.png">

그런데 문제가 있다. 10,000의 클라이언트와 연결을 맺고 수백만의 키들을 유지하고 있다고 생각해보자.  
이런 서버가 동작하려면 아주 비싼 CPU와 Memory가 필요할 것이다.  

그래서 Redis는 2가지 아이디어 구현을 통해 Memory CPU 사용량을 핸들링한다.  

1. 클라이언트의 리스트를 저장하는 single global table을 유지한다. 해당 테이블은 Invalidation Table 라고 부른다. entry max 만큼 저장할 수 있고, 새로운 키가 입력되면 키가 삭제된 것으로 여기며 클라이언트들에게 invalidation message 를 전송한다. 이 방식을 통해 클라이언트가 같은 키를 가지고 있더라도 메모리를 회수할 수 있다.  
2. invalidation table에 클라이언트 구조 포인터를 실제로 저장할 필요는 없다. 우리는 레디스 클라이언트의 고유 ID를 저장한다. 클라이언트와 연결이 끊기면 garbage collector가 메모리를 확보할 것 이다.
3. single key namespace를 사용한다. database number를 구분하지 않는다. 예를들어 database 2에서 foo를 캐싱하고, database 3에서 foo를 수정하더라도 invalidation message는 전송될 것이다. 이를 통해 memory usage를 절약하고 구현 복잡도를 낮췄다.

#### Two Connections mode
Redis 6이 지원하는 RESP3 프로토콜을 사용한다면, 같은 커넥션에서 쿼리와 invalidation message를 받을 수 있다.  
대부분의 클라이언트들이 invalidate message connection, data connection을 분리한다. 이를 통해 invalidation message를 client ID로 식별되는 다른 커넥션으로 redirect 할 수 있다. 다수의 data connection이 invalidation message를 하나의 connection으로 redirection 할 수 있다. 이 방법은 connection pooling을 구현할 때 유용하다. two connections model은 RESP2에서도 지원한다.

RESP2 로 구현한 example을 살펴보자.
example은 tracking redirecting을 활성화, asking for a key, modified key의 invalidation message를 받는 시나리오이다.  

시작하기위해 일단 첫번째 커넥션을 연다. 이 커넥션은 invalidation, connection ID 요청, invalidation message를 받는 Pub/Sub channel 등 여러 용도로 사용된다.  

```shell
(Connection 1 -- used for invalidations)
CLIENT ID
:4
SUBSCRIBE __redis__:invalidate
*3
$9
subscribe
$20
__redis__:invalidate
```

data connection에서 이제 tracking 을 활성화하자

```shell
(Connection 2 -- data connection)
CLIENT TRACKING on REDIRECT 4
+OK

GET foo
$3
bar
```

클라이언트는 로컬 메모리에 "foo" => "bar" 를 캐싱하게 되었다.  
이제 다른 클라이언트에서 "foo" 를 변경해보자

```shell
(Some other unrelated connection)
SET foo bar
+OK
```
그러면 invalidations connection은 invalidation message를 받게 된다.

```shell
(Connecction 1 -- used for invalidations)
*3
$7
message
$20
__redis__:invalidate
*1
$3
foo
```

클라이언트에서는 캐싱 슬롯에서 해당 키가 있는지 확인하고 더 이상 유효한 데이터가 아니라고 판단하여 버린다.  
Pub/Sub 메세지의 세번째 원소는 single key가 아니라 array로 이루어진 single element라는 것에 주목해야한다.  array를 전송하면 모든 원소에 대해서 validate을 single message에서 할 수 있다. 

client-side caching을 구현하기 위해서는 RESP2, Pub/Sub connection의 이해가 중요하다. Pub/Sub과 REDIRECGT를 사용하여 Trick을 부리는 것과 같기 때문이다. 반면에 RESP3 를 사용하면 invalidation message가 connection에 의해서 전송된다.

#### Opt-in chaching
클라이언트는 선택한 키만 캐시할 수 있다. 이건 캐시를 추가할 때  더 많은 bandwidth가 필요하지만 서버에 저장되는 data의 양과 클라이언트가 수신하는 invalidation message는 줄어든다.  

`CLIENT TRACKING on REDIRECT 1234 OPTIN`



### broadcasting mode
서버가 클라이언트를 기억하지 않는다. 따라서, 서버의 메모리를 사용하지 않는다.  
대신에 클라이언트가 subscribe 하고 있으며 매칭되는 키를 다룰 때 마다 notification message를 받는다.  

...

## Redis pipelining
multiple command 들의 각각 response를 기다리지 않도록 해서 퍼포먼스를 향상 시키는 기술이다.  
이 기술의 문제점을 어떻게 해결했는지 알아보자

### Req/Res protocols and RTT(Routd-Trip Time)
Redis는 TCP의 Server - client model의 프로토콜을 사용한다. 그래서 요청마다 응답을 받아야만한다.  
이 네트워크 지연시간을 round trip time이라고 부른다. RTT에 의하면 100k/s 의 요청을 처리할 수 있다.  


### Pipelning
Req/Res 서버가 기존에 Req를 처리한 적이 없더라도 새로운 Req를 처리할 수 있도록 구현할 수 있다.  
이를 통해 multiple command를 대기시간 없이 서버로 전송하고 응답을 single step으로 읽을 수 있게 된다.  
이 기법을 pipelining이라고 부르고 아주 넓게 사용되고 있다. 예를들어 POP3 프로토콜의 등장으로 이메일 다운로드 프로세스가 획기적으로 개선된 사례가 있다.  

raw netcat utility 로 pipelining을 구성하는 예제이다.
```shell
$ (printf "PING\r\nPING\r\nPING\r\n"; sleep 1) | nc localhost 6379
+PONG
+PONG
+PONG
```

위의 예제는 모든 call마다 RTT가 소요 되지 않는다. 한번에 세개의 커맨드를 날린 것이다.  
주의할 점은 pipelining으로 명령어를 전송할 때는 서버에 명령어 queue를 유지해야하고 메모리를 사용한다. 명령어가 많아지면 주의하자.  


요청이 많아지면 RTT도 많지만 system call도 증가하고 context switching 비용이 증가한다.

## keyspace notifications
클라이언트는 Pub/Sub 채널을 구독하여 Redis data set에 발생하는 이벤트들을 수신할 수 있게 해준다.  
Pub/Sub 연결이 끊겼다가 재연결되면 그동안 못받은 이벤트들을 다시 받을 수 있다.  

### Type of events
예제를 살펴보자
```shell
PUBLISH __keyspace@0__:mykey del
PUBLISH __keyevent@0__:del mykey
```
database 0에 존재하는 mykey 에 DEL 명령어를 보냈을 때의 이벤트를 구독한다.

## Pub/Sub
일반적인 pub...sub.. 

## Transactions
group of command을 single step 으로 트랜잭션으로 실행한다.  
명령어를 독립적으로 실행되는 것을 보장한다.  

EXEC 명령어가 트리거가 되어 모든 명령어를 트랙잭션안에서 수행하게 된다.  

### Usage

```shell
> MULTI.            <<< 트랜잭션 시작
OK
> INCR foo
QUEUE
> INCR bar
QUEUE
> EXEC              <<< 명령어 수행
1) (integer) 1
2) (integer) 1
```

### Errors inside a transaction
* command가 문법적 오류가 있거나, 서버의 설정보다 큰 연산이 발생하는 커맨드일 경우에 EXEC 실행 전에 queue가 된다.
* EXEC 실행 이후에 명령어가 실패하면, key와 value가 잘못된 경우이다.  

이 예제를 살펴보자

```shell
MULTI
+OK
SET a abc
+QUEUED
LPOP a
+QUEUED
EXEC
*2
+OK
```

주목할 점은 command가 실패하더라도 나머지 명령어가 전부 실행된다는 점이다.

추가로 이 예제를 살펴보자  
```shell
MULTI
+OK
INCR a b c
-ERR wrong number of arguments for 'incr' command
```

이 예제는 INCR 단일 명령어이기 때문에 큐잉되지 않는다.

### What about rollbacks
Redis는 rollback을 지원하지 않는다.

### 트랜잭션 중단
트랜잭션을 시작하고 DISCARD 명령어로 중단할 수 있다.

```shell
> SET foo 1
OK
> MULTI
OK
> INCR foo
QUEUED
> DISCARD
OK
> GET foo
"1"
```
