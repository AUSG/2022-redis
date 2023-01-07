# Bulk Loading

기존재하는 데이터를 redis로 들여올 때 사용하는 방법

- bulk loading을 해야할 때에는 다음의 이유들로 일반 클라이언트를 사용하지 않는것이 좋다.
  1. 커맨드 하나를 수행할때마다 RTT가 추가된다. 파이프라이닝을 사용할 수도 있겠지만, bulk loading할 때에는 많은 데이터들이 빠르게 잘 입력되고 있는지 insert와 동시에 확인되어야 한다(read replies).
  2. 대부분의 클라이언트는 non-blocking I/O를 지원하지 않으며, 처리량(throughput)을 최대화하는 방향으로 replies를 효율적으로 파싱하도록 구현되어있지 않다.

따라서 일반적으로 Redis에 대량의 데이터를 들여오기 위해서는 Redis Protocol을 포함하는 raw format의 text file을 생성하는 것이 낫다.



## How-to

- 2.6 미만의 버전에서는 nc(netcat)를 이용하여 Redis 머신에 직접 txt io를 feeding하는 방식으로 수행한다. 다만 이 방식은 언제 모든 데이터가 전송될지, 또 redis가 해당 데이터를 에러없이 잘 입력했는지 확인할 수 없기 때문에 권장되지 않는다.

- 2.6 이상의 버전에서는 redis-cli에 내장된 `pipe mode`를 사용한다.

  - ```bash
    cat data.txt | redis-cli --pipe
    ```

  - 이 방식은 인스턴스에서 받은 오류를 stdout으로 리다이렉팅 해준다.



### Redis Protocol 생성하기

디테일은 [링크된 문서](https://redis.io/topics/protocol)를 참고한다.

전체 형태는 다음과 같은 일관적인 패턴을 따른다.

```xml
*<args><cr><lf>
$<len><cr><lf>
<arg0><cr><lf>
<arg1><cr><lf>
...
<argN><cr><lf>
```

- `<cr>` 은 \r을, `<lf>`는 \n을 의미한다.



적당한 언어를 이용하여 이 형식에 맞는 문자열 txt를 생성한 뒤 이를 redis-cli로 전달하면 된다.



## How the pipe mode works

- redis-cli --pipe는 서버에 최대한 데이터를 빨리 보내고, 동시에 읽을 수 있는 응답 데이터를 파싱 시도한다.
- stdin에 더 입력된 데이터가 없다면 서버에서 클라이언트로 `ECHO` 커맨드로 랜덤한 20바이트의 string을 보내는데, 클라이언트에서는 20바이트의 랜덤 문자열을 reply로 받았을때 마지막 command까지 모두 보낸것인지 확인해주어야한다.
- 최종적으로 모든 command를 전달했으면, 마지막으로 전달받은 20바이트의 랜덤 문자열을 다시 반환하여 완료되었음을 알린다.



# Distributed Locks

분산락은 다중 프로세스가 mutually exclusive way로 리소스를 공유할 때 자주 사용되는 패턴이다. Redis로 분산락 매니저(DLM, Distributed Lock Manager)를 구현한 라이브러리나 아티클은 많지만, 각기 다른 방식으로 구현된 편이고, 조금 더 복잡한 방식으로 구현했을 때 얻을 수 있는 장점들을 포기하고 심플하게 구현된 경우가 많다.



이에 Redis는 공식적으로 **`Redlock`**이라고 불리는 알고리즘을 제안하여 이를 사용하는 것을 추천하고 있다.

- Redlock의 구현체는 언어별로 여럿 있는 경우가 있으니 참고하자.



## Design

### Guarantees

분산락을 효율적으로 이용하기 위해 보장되어야하는 최소한의 세가지 성질이 있다.

1. 안전성 : 상호 배제(Mutual Exclusion)
2. 생존성 A : 데드락에서 자유로울 것. 리소스를 점유한 클라이언트가 터지거나 쪼개지더라도, 데드락이 발생하지 않고 lock을 획득할 수 있어야 한다.
3. 생존성 B : 내결함성(Fault tolerance). 대부분의 레디스 노드가 떠있는 한 클라이언트는 lock을 획득하고 풀어줄 수 있어야 한다.



### Failover 기반 전략만으로는 충분하지 않다.

대부분의 레디스 기반 분산락 라이브러리들이 가지는 문제점을 살펴보자. 이들 구현체는 락을 획득하고 해제하는 과정을 TTL을 가진 키를 생성하고 삭제하는 방식으로 구현한다.

- 락을 획득 : TTL을 가진 키 생성
- 락을 해제 : 키 삭제
- 해제하지 않는다면 TTL에 따라 만료됨으로써 락을 해제(생존성 A 원칙)

이 방식은 다음과 같은 문제를 가진다.

1. 레디스 마스터 노드가 다운되었다면 키를 획득하고 해제할 수 없다. -> *SPoF 문제*
2. replica를 승격시켜 마스터 노드를 대신한다.
   - 이 때 replica는 기존 마스터 노드의 asynchronous한 복제본이다.
   - 따라서 race condition이 발생할수 있다. 
     - 클라이언트 A가 마스터 노드에서 락을 획득한 뒤, 레플리카로 이 락에 대한 쓰기 요청이 복제되기 전에 마스터가 터져서 레플리카가 승격된다.
     - 클라이언트 B는 락이 없는 마스터노드에 요청을 할 수 있으므로, 해당 락을 획득할 수 있다. -> *안전성 원칙(Mutual Exclusion) 위반*

이러한 경우는 노드가 다운된 상태에서 여러명의 클라이언트가 동시에 락을 점유해도 되는 특수한 상황이 아니라면 사용할 수 없다.



싱글 인스턴스로 락을 구현하려고 할때 올바른 방법은 다음과 같다.

- 클라이언트가 리소스에 대한 락을 획득할 때, 해당 락의 value값을 임의의 값으로 지정해두고, 이 값을 기억한다.
- 어떤 클라이언트든 해당 락을 해제하기 위해서는 먼저 락의 value값을 해당 클라이언트가 알고 있는지 검사해야한다(문자열 매칭)
  - 락을 획득한 클라이언트만이 해당 값을 기억하고 있으므로, 그 클라이언트만 락을 풀 수 있게 된다.
- 이 때 임의 값은 random string이어도 좋고, 클라이언트 id와 timestamp를 조합한 방식으로 구현해도 좋다. 완벽하게 safe하지 않을수는 있지만, 대부분의 경우에 잘 동작할 것이다.

여기에 추가하여 key에 락 유효시간(lock validity time)을 두어 락을 점유한 후 일정 시간이 지나면 풀리도록 구현할 수 있다(일종의 TTL 개념).



이러한 방식은 레디스가 싱글 인스턴스인 상황에서는 잘 작동한다. 그러나 분산시스템에서는 조금 더 복잡한 과정을 거쳐야한다. 분산 시스템에서는 독립적인 Redis 노드가 N개 존재하기 때문이다.



## Redlock 알고리즘

레드락 알고리즘은 N개의 레디스 마스터가 있는 분산 환경일 때 락을 구현하기 위한 방식이다. 잠금을 획득하기 위해 위에서 설명했던 싱글 인스턴스 상황에서의 락 획득 방식을 조금 변형하여 사용한다.

1. 현재 시간을 밀리초 단위로 timestamp를 찍어 둔다.
2. 모든 인스턴스에서 동일한 키 및 임의 값을 사용하여 순차적으로 락을 획득하려고 시도한다. 이 때 각 인스턴스의 락 획득 제한 시간(timeout)은 락 유효시간에 비해 훨씬 더 적어야한다.
   - 락을 획득하는 시간동안 결국 다른 클라이언트의 요청은 제한되는 것이기 때문에, 이 시간은 최대한 줄여야한다.
   - 만약 다운된 노드가 하나라도 있다면, 락 획득 시간이 길어질 수 있으므로 최대한 빨리 다음 노드로 락 획득을 넘어가야 하기 때문.
3. 과반수의 락을 획득했고 이 때 락을 획득하는 총 시간(최종 시간 - 1번의 timestamp)이 락 유효시간보다 작으면 락을 획득한 것으로 간주한다.
4. 이 때 락 유효시간은 [디폴트 락 유효시간 - 락을 획득하는데 든 전체 경과 시간]이다.
5. 만약 분산 락을 획득하지 못했을 경우 락을 획득하지 못한 인스턴스를 포함하여 모든 인스턴스의 락을 해제한다.



이 알고리즘은 consistency, correctness가 보장되지 않는 허점을 가지고 있는데, [Designing Data-Intensive Applications(데이터 중심 애플리케이션 설계)](https://dataintensive.net/)를 쓴 마틴 클렙만이 [지적한 글](https://martin.kleppmann.com/2016/02/08/how-to-do-distributed-locking.html)이 있다. 분산시스템에서 시간이 동기화되어있지 않다면 발생할 수 있는 문제인데, 이 문제를 해결하기 위해 fencing token을 발급하여 해결할 수 있다, 그 방식은 위의 글에 나와있으니 참고하자.

이 지적에 대해 [레드락 설계자가 반박한 글](http://antirez.com/news/101)도 있다.



[사견]

레디스 문서에서는 다소 모호하게 distributed system이라는 단어만 사용하여 마치 클라이언트가 분산 환경일때 사용하는 락처럼 착각할수도 있는데, (적어도 내가 해석한 바로는) 레드락은 분산락의 subset 개념이다.

- 분산 락은 여러 클라이언트를 가지는 분산 시스템에서 공유하는 락을 구현하는 방법이다.
- 레드락은 분산 락의 일종이다.
  - 정확히는, N개의 Read/Write 마스터 DB 노드를 가지는 상황에서 분산 락을 구현하는 방식이다.
  - 레드락 알고리즘은 레디스 뿐만 아니라 다른 Disk DB등에도 적용될 수 있다.



그런데 이 방식에서 가정하고 있듯 N개의 multiple master server node가 있다면, 그 모든 node에 동일한 키 정보를 가지도록 N번의 순차적인 요청을 보내 잡는 레드락 방식은 그다지 효율적이지 않아 보였다. 결국 1개의 키를 N개의 노드에 동일하게 보관한다는 말이기 때문이다.

정확히 이유는 알 수 없지만,

- 네트워크 레이턴시를 고려하여 세계 각지에 동일한 Redis 클러스터를 구축해야한다던가
- 마스터 노드가 죽었을 때 레플리카가 싱크를 맞추고 승격되는 단 몇초간의 차이도 용납할 수 없는 중요한 서비스의 경우 가용성을 위해

여러 마스터 노드를 띄워야할 가능성이 있지 않을까 추측해본다.

- [Redlock 알고리즘 설계자이자 Redis 개발자 antirez의 아티클](http://antirez.com/news/101)

  > ```
  > Redlock is a client side distributed locking algorithm I designed to be used with Redis, but the algorithm orchestrates, client side, a set of nodes that implement a data store with certain capabilities, in order to create a multi-master fault tolerant, and hopefully safe, distributed lock with auto release capabilities.
  > ```

  여기에서 redlock의 목적을 implement a data store with certain capabilities, fault tolerant (+ safe)라고 언급하고 있다. 

- [[Question]Why do we need distributed lock in Redis if we partition on Key?](https://github.com/redis/redis/issues/9651)



### Retry on Failure

과반수의 락을 획득해야하는 레드락 알고리즘의 특성상 동일한 리소스에 대해 lock을 획득하려는 클라이언트들이 여럿 있으면 위험하므로, 이를 비동기화 하기 위해 요청이 실패할 시에 랜덤한 시간을 지연시키도록 구현해두어야한다.

- 요청이 다시 동시에 몰린다면 다음번에도 lock을 잡고자하는 클라이언트들이 경쟁하여 요청이 실패할 가능성이 있다.



### 성능, 장애 복구 및 fsync

- fsync 기능은 레디스의 현재 상태를 영속적으로(persistently) 보관하기 위하여 disk DB처럼 file에 기록해두는 기능이다.
- 일부 레디스 노드가 죽었을 때 기존의 분산락 정보를 잃어버린 상태로 재시작한다면, 다른 클라이언트가 분산 락을 또 잡게되는 불상사가 일어날 수 있다.
  - 이론적으로는 이를 막기위해 `fsync=always` 옵션을 켜두어야한다. 물론, 이 방식은 큰 동기화 오버헤드를 수반한다(인메모리 DB를 디스크 DB처럼 매번 동기화시키는 꼴이니...)



---



# Secondary Indexing

레디스는 key-value store처럼 보이지만, 실제로는 다양한 자료 구조를 지원하는 서버이므로 composite 인덱스 등을 포함하여 다양한 종류의 보조 인덱스를 생성할 수 있다.

몇가지 데이터 구조를 통해 레디스에서 보조 인덱스를 구현하는 방식에 대해 알아보자.

## sorted set의 숫자 인덱스

Redis에서 생성할 수 있는 가장 간단한 보조 인덱스 형태로, double precision float 형태의 score를 인덱스로 사용하는 것이다. 이때 오름차순으로 정렬된다.

score가 double precision의 float형태이기때문에, 순정 sorted set으로 구현할 수 있는 인덱싱 정밀도는 해당 수준(-2^53 ~ + 2^53)에 한정된다.



## object ID와 연결한 인덱스 만들기

object에 직접적으로 score index를 연결하여  sorted set을 구현하는 방식보다, object의 일부 필드(예를 들어 ID)를 이용하여 해당 필드와 score용 인덱싱 필드를 연관지어서 sorted set을 만들어두는 방식도 있다. 이 경우 인덱싱에 연결된 필드(ex-ID)가 변경하지만 않으면 score index를 변경하면서 object는 건드리지 않을 수 있다.

```bash
HMSET user:1 id 1 username antirez ctime 1444809424 age 38
HMSET user:2 id 2 username maria ctime 1444808132 age 42
HMSET user:3 id 3 username jballard ctime 1443246218 age 33
```



## Turning multi dimensional data into linear data

sorted set의 score는 값이 하나뿐이라, 만약 여러 필드의 데이터를 기준으로 인덱싱하여 다차원적으로 정렬하고 싶다면 이를 선형적으로 바꾸는 과정을 거쳐야한다.

- ex -  [Redis geo indexing API](https://redis.io/commands/geoadd/)에서는 위도/경도의 2개 데이터값을 합쳐 정렬하기 위해 Geo Hash라는 방식을 사용하여 선형변환한다.
- Redis로 다차원 데이터를 정렬하는 여러 라이브러리들이 언어별로 존재한다.



# Redis Pattern Examples

Redis를 애플리케이션 DB로 삼아 트위터 기능을 클론 설계함으로써 레디스를 사용하는 여러 방식들을 알아볼 수 있다.

- [PHP 구현체 - Retwis](https://github.com/antirez/retwis)
- [Java 구현체 - Retwis-J](https://docs.spring.io/spring-data/data-keyvalue/examples/retwisj/current/)



구구절절한 구현이 많으므로 특징적인 부분들만 간략하게 정리하겠다.

- Key-value 저장소를 사용할 때에는 관계형 DB처럼 역참조가 어려우므로, **모든 정보에 기본키로만 접근한다**는 원칙을 가지고 설계해야한다.
  - 따라서 만약 value를 기준으로 key를 가져와야하는 경우가 생긴다면 관계를 뒤집어서 value => key, key => value로 동일한 데이터를 집어넣어 주는것이 원칙이다.
- 게시글 피드(업데이트)같은 정보들은 최신순으로 정렬해야하고, 페이지네이션이 구현되어야한다.
  - 따라서 큐/스택처럼 사용할 수 있고 RANGE로 범위 쿼리가 가능한 List를 사용한다.
- 사용자 인증시 세션 정보를 레디스에 보관하여 유효성을 검사할 수 있다.
- 서비스 확장성을 위해서는 샤딩 혹은 Redis 클러스터 도입을 고려한다.


