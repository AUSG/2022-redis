# Use Cases

## Client-side Caching

![[https://linuxhint.com/redis-client-side-caching/](https://linuxhint.com/redis-client-side-caching/)](seongik_assets/Untitled%201.png)

[https://linuxhint.com/redis-client-side-caching/](https://linuxhint.com/redis-client-side-caching/)

### 문제점

- DB는 일반적으로 단일 장애점이 되기 쉽고, scale out이 쉽지 않다는 점에서 병목의 중심이기 때문에 최대한 DB 요청을 줄이는 것이 좋음
    - 여기서 DB는 애플리케이션 서버와 별개의 노드위에 떠 있는 On-disk DB 혹은 In-memory DB 서버를 모두 일컬음
        - On-disk DB : 오라클, MySQL, Postgres, Mongo DB 등
        - In-memory DB : Redis, Memcached 등
- SNS같은 사례에서는 수정되는 post의 수가 적을뿐만 아니라, 최근 Post나 팔로워가 많은 일부 유명 유저들의 Post가 전체 Post보다 매우 인기가 많은, 이른 바 hot key가 존재함
- 당연히 네트워크를 타는 DB보다 클라이언트(즉, 여기서는 서버 노드)에 직접 정보를 캐싱해 둘 수 있지만, 오래된 데이터를 무효화해야한다는 악명높은 Cache Invalidation 문제가 존재
    
    > *There are only two hard things in Computer Science: cache invalidation and naming things.*
    > 
    > 
    > *- Phil Karlton*
    > 
    - Cache Invalidation 방법으로 간단한 방식으로는 TTL을 두는 방식을 사용할 수 있다. 그냥 시간이 지나면 만료된 정보로 여기고 다시 가져오는 방식.
    - 좀 더 복잡한 방식으로는 Redis Pub/Sub을 사용하여 여러 노드들의 캐시를 무효화할 수 있지만, Bandwidth 측면에서 비용효율적이지 않을 수 있다. 일부 Subscriber(즉 서버 노드)만 해당 메모리를 캐싱해두고 있고, 다른 노드들은 가지고 있지 않음에도 모든 Subscriber에게 publish 해야하기 때문.
        - Publish 자체도 데이터를 변경하는 애플리케이션 쿼리이므로 CPU시간이 많이 소모?

### 해결책

Redis 6부터는 Client-side Caching을 효율적으로 지원하기 위해 2가지의 **Tracking** 방식을 제공한다.

1. Default 모드
2. BroadCasting 모드

![[https://meetup.toast.com/posts/245](https://meetup.toast.com/posts/245)](seongik_assets/Untitled%202.png)

[https://meetup.toast.com/posts/245](https://meetup.toast.com/posts/245)

### Tracking - Default 모드

- 클라이언트가 tracking을 활성화 하는 시점부터 connection lifetime 동안  Redis 서버 측에서 클라이언트마다 액세스(Get)한 키 값을 메모리에 들고 있는다. 그러다가
    1. [해당 키의 수정요청이 일부 클라이언트 노드에서 발생하여 Redis에 SET 요청이 들어오거나](https://www.notion.so/21-08-08-Caching-bf721685d6db4b018d80eb6b492bd60c)
    2. expire time을 넘기거나
    3. maxmemory 정책을 넘기면
        1. `tracking-table-max-keys` 파라미터를 사용하며, 디폴트는 100만개의 키값 저장
        2. Invalidation Table : 액세스한 키값과 클라이언트를 저장해두는 테이블
    
    invalidation message를 보내어 클라이언트 캐시를 무효화시킨다.
    
- Bandwidth 측면에서 이득 - 해당 캐시를 메모리에 들고있(다고 추정되)는 클라이언트에게만 메시지를 보내므로
- Memory를 희생 - Redis 서버에 액세스된 모든 키값을 들고 있어야하므로
- 로컬 캐싱해두지 않을 데이터도 GET 요청시에 키값을 Invalidation Table에 등록될 수 있는데, 이 경우 불필요한 Invalidation Message가 증가함
    - 이를 막기 위해 Tracking 옵션을 켜둘 때 `OPTIN` 옵션을 같이 켤 수 있음
        - `CLIENT TRACKING on REDIRECT 1234 OPTIN`
        - 이 이후에는 GET 요청 직전에 `CLIENT CACHING YES`라는 명령을 같이 보내야 직후의 명령이 추적됨
            - 개체 캐싱시에는 명령어를 두번 보내므로 더 많은 Bandwidth가 필요하지만, 사용하는 Memory와 Invalidaiton Message 양을 줄일 수 있음.

### Tracking - BroadCasting 모드

- Default 모드와 달리 클라이언트가 액세스한 키 집합을 기억하지 않고, 그냥 대신 클라이언트가 직접 특정 prefix의 키(ex - `object:`, `user:`)를 subscribe하여 해당 prefix의 키가 변경될때마다 알림(Invalidation Message)을 받음
    - 이 때 prefix는 0개 혹은 1개만 사용 가능
        - 0개일 경우 모든 키에 대해 알림을 받게 됨
        - 2개 이상일 경우 매칭되는 prefix들중 한 prefix만 알림을 받게 됨
    - Invalidation Table은 아예 사용하지 않고, 대신 Prefix table을 만들어서 사용함
        - Prefix Table : 키값 전체가 아니라 prefix와 매칭되는 키를 가진 클라이언트를 저장해두는 테이블
        - Prefix가 많을수록 CPU 사용량이 높아진다.
    - Prefix 단위로 알림을 보내므로, 클라이언트 A가 직접 변경하여 SET한 키인데도 Invalidation Message가 와서 무효화되어 로컬 캐시를 불필요하게 한번 더 write해야할 수 있다. 이를 막기위해 `NOLOOP` 옵션을 켜둔다.
- Memory를 절약 - Redis 서버 메모리를 거의 사용하지 않음
- Bandwidth를 희생 - prefix를 가지고 있는 모든 클라이언트가 정확한 해당 키의 캐싱을 하지 않고 있더라도 알림을 받음

### 주의할 점

- 빠르게 변경되는 키나, 드물게 요청되는 키는 되도록 캐시하지 말고, 합리적인 속도로 키가 변경되는 경우에 클라이언트 캐싱을 적용하는 것이 효율이 좋음
- 클라이언트-Redis 소켓 연결이 끊기는 경우를 대비해서 로컬 캐시를 flush하도록 처리해둘것.
- 로컬 캐시에 TTL을 설정해두는 것이 redis db 커넥션이 끊기는 장애나 로컬에 오래된 정보를 저장해두는 경우를 방지하기에 좋음.
- 클라이언트가 로컬 캐싱에 사용하는 메모리 총량을 반드시 제한해두어야함.
    - [캐시 교체 알고리즘](https://www.notion.so/21-08-08-Caching-bf721685d6db4b018d80eb6b492bd60c) 사용 등의 방식
- Race condition 방지
    - GET 요청 → Invalidation Message 수신 → GET 응답 반환 시 수정된 캐시가 아니라 기존 캐시를 조회하게 될 수 있음
    - 이런 경우를 피하기 위해 GET요청 전에 로컬 캐시 키를 잠깐 복사해두고, Invalidate 요청이 들어왔을때 로컬 캐시 원본을 즉시 삭제하도록 하면 GET 응답이 뒤늦게 오더라도 기존 캐시값으로 덮어씌워져서 남는 문제가 없음.
        - 예시
            
            ```bash
            # [D] Data Connections | [I] Invalidation Connections
            
            # Race Condition
            [D] client -> server: GET foo
            [I] server -> client: Invalidate foo (somebody else touched it)
            [D] server -> client: "bar" (the reply of "GET foo")
            
            # Avoid Race Condition
            Client cache: set the local copy of "foo" to "caching-in-progress"
            [D] client-> server: GET foo.
            [I] server -> client: Invalidate foo (somebody else touched it)
            Client cache: delete "foo" from the local cache.
            [D] server -> client: "bar" (the reply of "GET foo")
            Client cache: don't set "bar" since the entry for "foo" is missing.
            ```
            

## Pipelining

각 명령어 대한 응답을 기다리지 않고 여러개의 커맨드를 한번에 발행함(issuing)으로써 성능을 개선하는 방법

![[https://hamedzarei.github.io/Redis](https://hamedzarei.github.io/Redis)](seongik_assets/Untitled%203.png)

[https://hamedzarei.github.io/Redis](https://hamedzarei.github.io/Redis)

### 문제점

- Redis는 기본적으로 Request-Response를 수행하는 클라이언트-서버 모델로, TCP 커넥션 소켓을 맺어 요청을 보내고 주고받음.
- 기본적으로 하나의 Request에 하나의 Response가 따라붙도록 되어있는데, Redis 서버 자체는 결국 클라이언트와 서로 다른 머신위에 떠있어 요청을 주고받을때마다 네트워크 통신 시간이 필요
    - 이러한 Request-Response 루프에 걸리는 시간을 RTT(Round Trip Time, 왕복시간)라고 부름
- Redis 서버를 사용하는 효율이 RTT 시간이 증가함에 따라 나빠질 수 있음

### 해결책

![Redis 공식문서의 파이프라이닝 효율 벤치마크 - [https://redis.io/docs/manual/pipelining/](https://redis.io/docs/manual/pipelining/)](seongik_assets/Untitled%204.png)

Redis 공식문서의 파이프라이닝 효율 벤치마크 - [https://redis.io/docs/manual/pipelining/](https://redis.io/docs/manual/pipelining/)

Redis는 **버전과 관계없이** Pipelining 기술을 지원하며, response을 기다리지 않고 request를 여럿 보낸 후 최종적으로 단 한번 response를 읽는 방식

- 이러한 방식은 RTT같은 네트워크 레이턴시 관점뿐만 아니라 소켓 I/O를 수행하는 비용을 고려해보았을때 합리적인 방식
    - 소켓 I/O를 수행할 때 매 응답마다 read(), write(), syscall 호출이 수행되는데, 파이프라이닝을 사용하면 한번의 read() / write() 요청으로 여러 응답값을 한번에 전달가능하므로.
- Redis 2.6부터 사용가능한 Redis Scripting(Lua 기반 스크립팅)을 사용하면 여러 명령이 포함된 파이프라이닝을 모아두고 사용할 수 있음.

### 주의

- Pipelining을 이용하여 클라이언트가 명령을 보낼때, 서버에서는 이 다수의 명령을 Queue에 옮겨두고 처리하는데, 이 Queue를 구성하는데에도 Redis 서버의 메모리를 사용한다.
    - 따라서 아주 다량(ex-수십K 이상)의 명령을 보내야하는 경우 한번에 명령을 모두 모아 파이프라이닝하는 것이 아니라, 적절한 배치로 끊어서 response를 한번 확인하고 다시 보내는 형태가 적절하다.
- Redis 서버와 클라이언트가 같은 머신에 존재할때에도 Loopback Interface라도 느린 것 처럼 벤치마크가 나올 수 있다.
    - RTT 관점에서 이득을 볼것이라고 생각하겠지만, 실제로는 시스템의 각 프로세스들이 항상 실행되고있는것이 아니라 필요할 때마다 Redis 서버 프로세스와 클라이언트 프로세스를 커널 스케줄링 시켜 작업을 수행하기 때문에, 네트워크 레이턴시가 없더라도 시간효율 면에서 손해를 보기 때문.

## Keyspace notification

Redis Key, Value 데이터들의 변경 이벤트 알림을 Pub/Sub으로 구독하여 실시간 모니터링 할 수 있음

수신가능한 이벤트의 종류

- 특정 키에 영향을 미치는 모든 커맨드
- LPUSH 명령을 수신하는 모든 키
- database 0 에서 만료되는 모든 키

### 작동 방식

![[http://intro2libsys.com/focused-redis-topics/day-three/caching-keyspace-notifications](http://intro2libsys.com/focused-redis-topics/day-three/caching-keyspace-notifications)](seongik_assets/Untitled%205.png)

[http://intro2libsys.com/focused-redis-topics/day-three/caching-keyspace-notifications](http://intro2libsys.com/focused-redis-topics/day-three/caching-keyspace-notifications)

- Keyspace notification은 CPU를 사용하므로 디폴트로 비활성화되어있지만, redis.conf 혹은 CONFIG SET에서 `notify-key-space-events` 설정시 활성화됨.
- 데이터 스페이스에 영향을 미치는 작업이 수행되었을 경우, 예를 들어 mykey라는 이름의 key를 database 0에서 DEL 했을때 다음 두가지 이벤트가 트리거됨(둘 중 하나만 트리거하도록 환경 설정 가능)
    1. mykey라는 이름의 키에 수행되는 모든 이벤트를 수신하는 `keyspace` 채널에 event 명을 메시지로 PUBLISH
    2. mykey라는 이름의 키에 수행되는 DEL 명령 이벤트만 수신하는 `keyevent` 채널에 event가 수행된 key name을 메시지로 PUBLISH
- 각 operation에 대한 event 정보는 [공식 docs](https://redis.io/docs/manual/keyspace-notifications/)를 참고한다.

### 주의할 점

- 실제로 명령이 수행되어 키가 수정된 경우에만 이벤트를 발행함. (존재하지 않는 키를 삭제시도하는 등 어떤 연유로든 실제 데이터셋이 변경되지 않으면 이벤트를 발행하지 않음)
- TTL이 있는 키는 Redis 내에서 다음 두가지 방식으로 만료된다.
    1. 키값에 액세스했는데 expire time이 만료되었다는 것이 확인되었을 때(semi-lazy mechanism)
        - 이 경우 키를 액세스하지 않으면 garbage가 쌓여 메모리를 계속 차지한다는 문제점이 존재한다.
    2. 백그라운드에서 돌아가는 워커가 만료된 키를 찾아서 삭제할 때
        - 초당 10회 이상 TTL이 있는 모든 키 중 임의의 20개를 뽑아서 expired된 모든 키를 삭제하고, 이 때 25% 이상의 키가 만료되었을 경우 이 과정을 즉시 한번 더 수행한다.
            - 이를 통해서 확률적으로 25% 이상의 메모리를 garbage가 차지하지 않도록 제한한다.
            - 다만 이 방식은 Redis 6에서 Radix를 사용하여 만료시간별 키 정렬을 함으로써 개선되었다고 추정됨
    - 즉, **TTL에 도달하자마자 바로 삭제되는것이 아니라, 실제로 키값이 삭제될 때 expire 이벤트를 발행함**
- 레디스 클러스터를 구성한다면, 각 노드들은 각자 키스페이스 subset에 대한 이벤트를 따로 생성함. 이 이벤트는 다른 노드에 자동으로 broadcast되지 않으므로 클러스터의 모든 이벤트를 수신하려면 클라이언트를 각 노드에 subscribe시켜야함.

## Pub/Sub

Redis를 이용하여 Channel을 구성하고, Publisher와 Subscriber를 두어 메시지를 중개하도록 할 수 있음.

![Untitled](seongik_assets/Untitled%206.png)

### 특징

- 키스페이스, database number의 제약이 없이 모든 수준에서 broadcast됨
    - 범위 지정하여 broadcast가 필요할 경우 channel 명에 env prefix(ex- `dev_`, `stage_`)를 붙여서 구별하는것이 컨벤션
- 패턴 매칭을 지원하므로, 해당 채널의 모든 메시지가 아니라 특정 패턴에 맞는 메시지만 구독할 수 있음
    - ex - `PSUBSCRIBE news.*`
- Redis 7.0부터 클러스터 모드에서 샤딩을 수행했을때 샤딩된 노드로만 메시지를 보내는 기능이 추가되었음

### 주의할 점

- Redis Pub/Sub은 fire-and-forget 방식이므로, Pub/Sub 채널이 끊어졌다가 다시 연결되는 동안 일어난 모든 이벤트는 소실된다.
    - **즉, 이벤트를 저장하지 않는다는 것이 다른 Pub/Sub 구현체들과의 핵심적인 차이점.**

## Transactions

여러개의 명령어를 합쳐서 단일한 EXEC요청을 수행할 수 있다.

![Untitled](seongik_assets/Untitled%207.png)

- `MULTI` 명령어를 통하여 트랜잭션을 시작하고, `EXEC`을 이용하여 트랜잭션을 묶어 수행하며, `DISCARD`로 트랜잭션을 flush한다.

### 특징

- 트랜잭션 내의 모든 명령은 다른 클라이언트의 요청과 절대 섞이지 않으며, serialize되어 순차적으로 실행된다(single isolated operation)
- EXEC 명령은 한번에 모든 명령을 trigger하기 때문에, EXEC이 호출되기 전까지는 트랜잭션 컨텍스트 상에서 서버 커넥션이 끊어지더라도 수행되지 않는다. 만약 append only file을 추가하는 과정에서 서버에 오류가 생겨 트랜잭션의 일부만 등록될 경우 `redis-check-aof`를 통해 파편화된 트랜잭션을 제거하도록 수정하는 작업을 수행할 수 있다.

### 트랜잭션 도중 오류가 발생하는 사례

1. EXEC 명령이 잘못된 명령이거나 메모리 제한 등에 걸려 실패하는 경우
    - Redis 2.6.5부터는 명령을 누적하는 과정에서 오류를 감지하고 EXEC을 거부하며, 트랜잭션을 discard한다.
2. 트랜잭션의 일부 명령에서 오류가 반환되는경우(잘못된 명령이 아니라, 제대로 수행되지 못하고 결과값을 에러로 리턴하는 경우)
    - 이 경우 트랜잭션 명령 대기열(QUEUE)에 존재하는 나머지 모든 명령어는 수행된다.

### Locking

- 트랜잭션 명령을 쌓는 동안 race condition이 발생하는것을 방지하기 위해 ***Optimistic Locking(낙관적 락)***을 사용한다.
    - `WATCH` 명령어를 이용하여 특정 키값에 대한 낙관적 락을 걸고, 만약 EXEC 전의 트랜잭션을 쌓는 과정에서 해당 키가 변경되면 EXEC시에 전체 트랜잭션을 중단하고 NULL 값을 반환한다.
        - WATCH는 일종의 조건형태로, 키가 수정되지 않았는지 조건부 확인 후 EXEC을 수행하도록 해준다.
        - 트랜잭션 내의 각 명령들은 큐잉된 상태일 뿐이므로 WATCH의 조건을 트리거하지 않는다.
    - 해당 명령을 다시 수행하기 위해서는 이번에는 해당 키가 수정되지 않기를 바라면서 동일한 작업을 반복해야한다. 이처럼 트랜잭션 충돌 혹은 race condition이 발생하지 않을 것을 가정하고 EXEC(on-disk DB에서는 commit)을 수행한 뒤 실패하도록 하는 방식이므로 낙관적 락이라고 할 수 있음.

### 주의할 점

- Redis는 일반적인 DB와 달리 성능과 simplicity에 영향이 가는것을 피하기 위해 **롤백을 지원하지 않는다**.

# 응용 사례

- 검색 자동완성 - sortedSet 활용
- 최근 인기있는 게시물(hot 게시물) 확인
- 유저 채널링

# Further Readings

- [Redis TTL의 작동 방식](https://www.pankajtanwar.in/blog/how-redis-expires-keys-a-deep-dive-into-how-ttl-works-internally-in-redis)

---

# Reference

[Using Redis](https://redis.io/docs/manual/)

[레디스 버전6 뉴피처와 주요 기능 테스트 : NHN Cloud Meetup](https://meetup.toast.com/posts/245)

[Redis server-assisted client side caching](http://redisgate.kr/redis/server/client_side_caching.php)

[https://linuxhint.com/redis-client-side-caching/](https://linuxhint.com/redis-client-side-caching/)

[Pub/Sub Introduction Redis](http://redisgate.kr/redis/command/pubsub_intro.php)
