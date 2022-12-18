# Redis Patterns

## Bulk Loading
Redis protocol과 data bulk write 해보자.  

### Bulk loading using the Redis protocol  
normal redis client로 bulk loading 하는 것은 좋은 생각이 아니다.  
1. 모든 명령어에 RTT 만큼 레이턴시를 가진다.  
2. 파이프라인을 통해 응답을 받는 동시에 새로운 명령어를 쓰면서 속도를 증가시킬 수 있다.  

아주 소수의 클라이언트만 논블록킹 I/O를 지원하며, 모든 클라이언트들이 최대 throughput 만큼의 응답을 처리할 수 있을정도로 효율적이지 않다.  
그러니까 Redis protocol을 사용하자.

다수의 명령어들이 있는 텍스트 파일을 netcat 을 이요해서 REDIS에게 먹여보자.

`(cat commands.txt; sleep 10) | nc localhost 6379 > /dev/null`

빠르긴 한데 신뢰할 수 없는 방법이다. 모든 명령어가 잘 적용되었는지 알 수 없고 에러 체크가 안되기 때문이다.  
2.6 이상에서는 redis-cli 라는 유틸리티 툴이 지원되며 pipe mode 를 통해 bulk loading을 효율적으로 할 수 있다.  

```shell
$ cat data.txt | redis-cli --pipe

---
---

All data transferred. Wating for the last reply...
Last reply received from server.
errors: 0, replies: 1000000
```

에러 메세지 구경하기, 커맨드, 파이프라인, netcat 퍼포먼스 비교하기 `#TODO` 

### Generating Redis Protocol
Redis protocol은 아주 쉽게 생성하고 파싱할 수 있다. 

```shell
*<args><cr><lf>
$<len><cr><lf>
<arg0><cr><lf>
<arg1><cr><lf>
...
<argN><cr><lf>
```

<cr> == "\r" (ASCII 13)
<lf> == "\n" (ASCII 10)

SET key value 커맨드를 만드는 예제를 보자

```shell
*3<cr><lf>
$3<cr><lf>
SET<cr><lf>
$3<cr><lf>
key<cr><lf>
$5<cr><lf>
value<cr><lf>

or

"*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
```

ruby 프로그램

```ruby
def gen_redis_proto(*cmd)
    proto = ""
    proto << "*"+cmd.length.to_s+"\r\n"
    cmd.each{|arg|
        proto << "$"+arg.to_s.bytesize.to_s+"\r\n"
        proto << arg.to_s+"\r\n"
    }
    proto
end

puts gen_redis_proto("SET","mykey","Hello World!").inspect

(0...1000).each{|n|
	STDOUT.write(gen_redis_proto("SET", "Key#{n}", "Value#{n}"))
}
```

```shell
$ ruby proto.rb | redis-cli --pipe
All data transferred. Waiting for the last reply...
Last reply received from server.
errors: 0, replies: 1000
```

### How the pipe mode works under the hood

* `redis-cli --pipe` 서버에게 최대한 빠르게 데이터를 전송한다.
* 데이터를 읽을 수 있게 됨과 동시에 파싱을 시도한다.
* STDIN에 더 이상 읽을 데이터가 없다면 랜덤한 20byte sting으로 구성된 ECHO 명령어를 전송한다. 이 명령어가 그대로 돌아오는 것이 보장되어야 한다.
* 응답을 받으면 랜덤 스트링을 매칭해본다. 매칭되면 성공으로 간주한다.


## Distributed Locks with Redis
서로 다른 다수의 프로세스들이 공유하는 분산 락을 구현해보자

다수의 라이브러리들이 DLM(Distributed Lock Manger)를 구현하고 있다. 모든 라이브러리들이 서로 다른 접근을 사용한다.  
canonical algorithm 인 Redlock에 대해서 알아보자. 아마도 vanilla single instance approach 보다 안전할 것이다.  

### Safety and Liveness Guarantees
분산락을 위한 세 가지 속성을 보장해야한다.

1. Saftey property: Mutual exclusion 이다. 반드시 하나의 클라이언트만 lock을 잡고 있을 수 있다.
2. Liveness property A: Deadlock free. lock을 잡던 클라이언트가 깨지더라도 항상 lock을 acquire 할 수 있어야 한다.
3. Liveness property B: Fault tolerance. Redis node가 up되면 클라이언트는 lock을 acquire / release 할 수 있다.  

### Why Failover-based Implementations Are Not Enough
우리가 이루고 싶은 것을 이해하기 위해서, 가장 {  } 한 레디스 분산락 라이브러리를 분석해보자.

레디스로 자원에 락을 거는 가장 쉬운 방법은 인스턴스에 키를 생성하는 것이다. 키는 대부분 TTL이 설정되어 있다.  
TTL이 초과되거나 키를 삭제하면 자원이 release된다.  

이 방식은 Redis master가 장애상황에 빠지면 전체 서버의 SPOF가 된다. Redis master가 장애에 빠지지 않도록 replication을 scale-out할 수 있지만, 이것은 mutual exclusion을 구현할 수 없다.

이 방식의 race condition을 살펴보자

1. Client A 가 master에 acquire한다.
2. master가 key를 replica에 복사하기 전에 죽는다.
3. replica가 master로 부터 promoted된다.
4. Client B가 Client A가 이미 lock해둔 자원에 대해 acquire한다. < SAFTEY VIOLATION


### Correct Implementation with a Single Instance
분산 락 알고리즘의 근-본을 살펴보자.

일단 냅다 acquire 커맨드
`SET resource_name my_random_value NX PX 30000``

1. NX == CREATE IF NOT EXISTS
2. PX == TTL 30000

기본적으로 아래 스크립트와 같이 안전하게 release 하려고 한다.
```shell
if redis.call("get",KEYS[1]) == ARGB[1] then # key가 존재할 때만
  return redis.call("del", KEYS[1])          # 제거한다.
else
    return 0
end 
```

random value는 20바이트의 random string이고, 이걸로 모든 클라이언트들이 sign할꺼야.

random value는 

1. /dev/urandom 에서 생성하거나 
2. UNIX timestamp(ms)를 사용하거나 
3. timestamp + ID를 사용하거나 
4. 니가 직접 만들어라.

어쨌든 이정도면 걍 single instance는 lock을 사용할 수 있음;;

### The Redlock Algorithm
5개의 Redis Master가 있다고 생각하자. 각 Redis Master는 서로 다른 VM 혹은 물리적으로 다른 컴퓨터에서 실행중이다.

acquire을 어떻게 할까
1. get current ms
2. 5개 인스턴스에서 동시에 같은 키, 이름, 랜덤 스트링으로 acquire하려고 한다. TTL이 10초면 timeout은 50 ms 이내여야한다. 그래야 하나의 클라이언트가 redis node를 오랫동안 점유하고 있지 않을 수 있다. 다른 인스턴스의 요청을 block 하게 두면 안된다.
3. step 3 current ms - step 1 ms 해서 elapse time을 계산한다. 이 시간이 TTL 보다 작으면 acquire 되었다고 판단한다. 그리고 과반 수 이상이 acquire 되어야 분산 lock이 성공한거다.
4. 만약 실패했다면 모든 인스턴스를 unlock 한다.

### Is the Algorithm Asynchronous?
모든 인스턴스의 시계가 동기화가 안되어 있다고 가정한다. 그래서 3단계에서 얻은 lock validity time 내에서는 인정해준다.

### Retry on Failure
random delay로 재시도한다. 부하를 줄이기 위해서 모든 인스턴스의 SET 요청을 멀티플렉싱으로 받는다.

### Releasing the Lock
TTL 을 기다리거나 DEL 해라.

### Safety Arguments
알고리즘이 안전한지 확인하려면 아래 조건을 만족해야 한다.

T1 = 첫 인스턴스가 키 설정 시간
T2 = 마지막 인스턴스 키 설정 시간
MIN_VALIDITY = TTL - (T2 - T1) - CLOCK_DRIFT

그리고 key가 설정되었으면 다른 클라이언트의 acquire은 성공할 수 없다.

### Liveness Arguments
Liveness 가 모지?
Liveness의 세가지 기능
1. auto release 이후에 다시 lock 할 수 있다.
2. auto release 외에도 release 되었다면 (terminated 등으로) TTL까지 기다릴 필요 없다. (잘 지워져야 된다는 뜻인가)
3. 클라이언트가 acquire를 retry 해야할 때, 원래의 elapsed time 보다 더 많은 시간을 기다려야 한다. 과부하 방지를 위해..


							   
### Performance, Crash Recovery and fsync
**Performance**
많은 명령어를 수행할 때도 적은 지연시간을 가지는 lock server가 필요하다면 redis를 사용하자.  
레디스는 논블록킹 소켓을 하나 두고, 모든 명령어를 때려 박는 멀티플렉싱 기법을 통해 이 요구사항을 해결했다.  

**Crash Recovery**
영속성 없는 레디스를 운영하고 있다고 가정하자.  

1. 클라이언트는 5개의 인스턴스 중 3개의 인스턴스에서 lock을 획득했다.  
2. lock을 획득한 인스턴스 중 하나가 재시작 되었다.  
3. 이때 3개의 인스턴스가 다시 동일한 리소스에 대해서 lock을 걸 수 있고, 다른 클라이언트도 다시 lock을 걸 수 있다. -> 상호배제 속성이 침해당함

**fsync**
레디스는 기본적으로 매 초 fsync를 하고 있다. 예상치 못한 종료로 인해 key를 잃지 않기 위해서이다.  
lock saftey를 보장받고 싶다면 `fsync=always` 설정을 추가하자. 대신 sync overhead가 늘어난다.  

###  Macking the algorithm more reliable: Extending the lock

...


## Secondary indexing
레디스는 전형적인 key-value 저장소는 아니다. 값에는 더 복잡한 자료형이 저장될 수 있다. API 레벨에서는 key로 조회할 뿐이다.  
Redis는 primary key access를 제공한다고 말하는 것이 맞다. Redis는 data structure server로서 다중 인덱스를 생성할 수 있다.  

이 문서에서는 다음과 같은 내용을 살펴볼꺼다.
* Sorted set 에서 ID, numerical field에 secondary index 걸기
* lexicographical range Sorted set으로 advanced secondary index, composite index, graph traversal index 만들기
* random index의 Set
* iterable index, last N item index의 List

인덱스 구현과 운영은 Redis의 advenced topic이다. 고성능이 필요한 유저들은 이 문서를 이해하도록 하자. (늅늅)

### Simple numerical indexed with sorted sets
가장 간단한 secondary index는 sorted set에 score를 부여하여 정렬한 형태이다.

sorted set을 만들고 조회 하는 예제를 살펴보자

```shell
# set 생성
ZADD myindex 256 Manuel
ZADD myindex 18 Anna
ZADD myindex 35 Jon
ZADD myindex 67 Helen

# 조회
ZRANGE my index 20 40 BYSCORE
```

### Using objects IDs as associated values
`이름 - 나이` 가 쌍인 데이터가 있다. Sorted Set에 이 데이터들의 ID를 저장하여 인덱스를 만들어보자.

ID라는 단일 키로 접근할 수 있는 유저 Hash가 있다.
```shell
HMSET user:1 id 1 username antirez ctime 1444809424 age 38
HMSET user:2 id 2 username maria ctime 1444808132 age 42
HMSET user:3 id 3 username jballard ctime 1443246218 age 33
```

위의 Hash에 나이에 대한 인덱스를 만들어 보자.
```shell
ZADD user.age.index 38 1
ZADD user.age.index 42 2
ZADD user.age.index 33 3
```

그리고 또 ZRANGE, BYSCORE를 사용하면 되겠지.

### Updating simple sorted set indexes
인덱스를 변경해야할 수 있다. 위의 예제에서는 유저들의 나이가 1년마다 증가한다.  
그럼 그냥 업데이트 해라

```shell
# 트랜잭션으로 묶는 것을 튜텬
HSET user:1 age 39
ZADD user.age.index 39 1
```

### Turning multi dimensional data into linear data
sorted set 인덱스는 단일 숫자 타입의 인덱스만 사용할 수 있다. 하지만 다차원 인덱스도 사용할 수 있다.  
다차원 인덱스를 선형 방식으로 사용해보자 

예를 들어서 geo index 는 geo hash 라는 테크닉을 통해 lat, lng 를 인덱스로 사용한다. 

아무튼 해싱해서 사용하라는 뜻인듯;;; 

#### Limits of the score
Sorted set은 score는 float 64 정확도를 가진다. 그래서 값에 따라서 다른 에러를 표현할 수 있다. 내부적으로는 지수 표현 방식을 사용하기 때문이다.  
인덱스는 항상 스코어가 항상 +-2^53 (9,007,199,254,740,992)에서 표현되어야 한다.

더 큰 숫자는 인덱싱 못한다. 그런 경우에는 lexicographical index를 알아봐라.

### Lexicographical indexed
Sorted set에 같은 score로 저장하면 memcmp() 함수로 바이너리 문자열을 비교해서 알파벳 순으로 정렬된다. 
전통적인 RDBMS 처럼 b-tree 를 데이터 구조로 사용한다. 그래서 꽤 괜찮게 인덱스할 수 있다.  

```shell
# same score
ZADD myindex 0 baaa
ZADD myindex 0 abbb
ZADD myindex 0 aaaa
ZADD myindex 0 bbbb

# fetch ordered lexicorgraphically
ZRANGE myindex 0 -1
1) "aaaa"
2) "abbb"
3) "baaa"
4) "bbbb"

# ZRANGE BYLEX
ZRANGE myindex [a (b BYLEX # a로 시작하는 문자열 중 a ~ b 만 포함하는 RANGE
1) "aaaa"
2) "abbb"

# ZRANGE BYLEX 2
ZRANGE myindex [b + BYLEX
1) "baaa"
2) "bbbb"
```

#### e.g 1 completion
검색창의 자동완성에 활용할 수 있다.

**navie approach**
걍 쿼리 날려버리기
`ZADD myindex 0 banana` 

**ZRANGE BYLEX**
검색창에 "bit" 를 입력했다고 치자.
`ZRANGE myindex "[bit" "[bit\xff" BYLEX LIMIT 5`
너무 많이 나올 수 있으니 LIMIT을 걸자

#### e.g 2 Adding frequency into the mix
선택 빈도를 추가해서 인기 있는 검색어를 추천해보자


```shell
# 셋팅
ZADD myindex 0 banana:1

# 조회
ZRANGE myindex "[banana:" + BYLDEX LIMIT 0 1
# 증가
ZREM myindex 0 banana:1
ZADD myindex 0 banana:2
```
동시성 문제가 있을 수 있으니 Lua script를 사용해라.

**example**
```shell
ZRANGE myindex "[banana:" + BYLEX LIMIT 0 10
1) "banana:123"
2) "banaooo:1"
3) "banned user:49"
4) "banning:89"
```

### Normalizaing strings for case and accents
"Banana", "BANANA", "Ba'nana" -> "banana" 로 정규화하고 싶다.
`ZADD myindex 0 banana:273:Banana # normalized:frequency:original` 

### Adding auxiliary information in the index
인덱스에 보조정보 추가하자 (정규화랑 뭐가 다름;;)
```shell
ZADD myindex 0 mykey:myvalue
ZRANGE myindex [mykey: + BYLEX LIMIT 0 1
1) "mykey:myvalue"
```

### Numerical padding
```shell
ZADD myindex 0 00324823481:foo
ZADD myindex 0 12838349234:bar
ZADD myindex 0 00000000111:zap

ZRANGE myindex 0 -1
1) "00000000111:zap"
2) "00324823481:foo"
3) "12838349234:bar"
```

### Using numbers in binary form
### Composite indexes
### Updating lexicographical indexes
### Representing and querying graphs using a hexastore
Sorted Set의 트릭이다. 나중에 찾아보도록 하자..

### Multi dimensional indexes
엔지니어링 비용이 박살날듯!

## Redis patterns example
트위터 클론하면서 패턴 예제를 살펴보자. RDBMS 뿐 아니라 NoSQL도 애플리케이션 개발에 아주 유효하게 사용할 수 있다.  
요거슨.. 따로 실습을 해보겠숨다..
