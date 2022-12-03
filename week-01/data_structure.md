# Redis data types
Redis는 data structure server 이다.  
Redis는 native data type의 collection을 제공하여 caching, queuing, event processing 문제를 해결할 수 있도록 돕는다.  


## Strings
byte sequence로 표현되는 가장 기본적인 data type이다.  
캐싱에 자주 사용되지만 counter를 구현해서 비트 단위 작업도 수행할 수 있다.

### Basic Commands
* Get, Set
  * SET
  * SETNX (Key가 없을 때만 저장한다. lock을 구현할 때 유용하게 사용할 수 있다.) ref: https://stackoverflow.com/questions/56589374/single-instance-redis-lock-with-setnx
  * GET
  * MGET (Multiple GET) 
* Counter
  * INCRBY (음수와 함께 사용하면 DECR 가능. 원자성 보장)
  * INCRBYFLOAT (소수 카운터)
* Bitwise 
  * chckout [bitmaps data type docks](https://redis.io/docs/data-types/bitmaps/) 

whole command: https://redis.io/commands/?group=string

### Examples
* store and then retrieve a string
```shell
> SET user:1 salvatore
OK
> GET user:1
"salvatore"
```

* Store a serialized JSON string and set TTL 100
```shell
> SET ticket:27 "\"{'username': 'priya', 'ticket_id': 321}\"" EX 100
```

* Increment a counter
```shell
> INCR views:page:2
(integer) 1
> INCRBY views:page:2 10
(integer) 11
```

### Limits
512MB 까지 저장할 수 있다.

### Performance
O(1) 이지만 SUBSTR, GETRANGE, SETRANGE 명령어는 조심해서 사용하자. 이 커맨드들은 O(N)이다.

### Alternatives
serialized string을 저장할거라면, Redis hashes, RedisJSON 사용을 고려해보자

## lists
Redis lists느 string value의 linked list이다.  
스택과 큐를 구현할 때 많이 사용하고, background worker system의 queue management에 많이 사용된다.

### Example
* FIFO queue
```shell
> LPUSH work:queue:ids 101
(integer) 1
> LPUSH work:queue:ids 237
(integer) 2
> RPOP work:queue:ids
"101"
> RPOP work:queue:ids
"237"
```
* FILO stack
```shell
> LPUSH work:queue:ids 101
(integer) 1
> LPUSH work:queue:ids 237
(integer) 2
> LPOP work:queue:ids
"237"
> LPOP work:queue:ids
"101"
```

* list 길이 확인
```shell
> LLEN work:queue:ids
(integer) 0
```

* 원자성을 보장하며 A 리스트에서 pop하고 B 리스트에 push 하기
```shell
> LPUSH board:todo:ids 101
(integer) 1
> LPUSH board:todo:ids 273
(integer) 2
> LMOVE board:todo:ids board:in-progress:ids LEFT LEFT
"273"
> LRANGE board:todo:ids 0 -1
1) "101"
> LRANGE board:in-progress:ids 0 -1
1) "273"
```

* LPUSH 호출 후 LTRIM을 호출하여 list length 제한하기
```shell
> LPUSH notifications:user:1 "You've got mail!"
(integer) 1
> LTRIM notifications:user:1 0 99
OK
> LPUSH notifications:user:1 "Your package will be delivered at 12:01 today."
(integer) 2
> LTRIM notifications:user:1 0 99
OK
```

### Limits
list의 길이는 2^32 - 1 을 초과할 수 없다.

### Basic commands
* LPUSH: head에 새로운 element를 추가한다.
* RPUSH: tail에 새로운 element를 추가한다.
* LPOP, RPOP: head/tail 의 원소를 제거하며 리턴한다.
* LLEN: list.length();
* LMOVE: 원자성을 보장하며 하나의 리스트의 원소를 다른 리스트로 전송
* LTRIM: list를 명시한 range만큼으로 줄여버린다.

#### Blocking commands
* BLPOP: LPOP과 같은 동작을 하는데, list가 비어 있으면 element가 push 될 때 까지 block 되거나 timeout 시간까지 block 된다.
* BLMOVE: 위와 같다.

### Performance
O(1) 인데, LINDEX, LINSERT, LSET 같은 커맨드들은 O(N)이다. 거대한 list를 다룰 때는 조심해서 사용해야 한다.

## Sets
set은 순서가 보장되지 않는 unique string 의 컬렉션이다. 
* IP 같은 unique item을 Tacking할 때 유용하게 사용 할 수 있다.
* 관계를 표현할 수 있다. (모든 유저의 권한 set)
* union, intersection 등과 같은 일반적인 set operation을 사용할 수 있다.

### Examples
* 123유저와 456 유저의 Favorite Book ID를 저장한다.
```shell
> SADD user:123:favorites 347
(integer) 1
> SADD user:123:favorites 561
(integer) 1
> SADD user:123:favorites 742
(integer) 1
> SADD user:456:favorites 561
(integer) 1
```
* 123 유저가 책 742, 299를 좋아하는지 체크
```shell
> SISMEMBER user:123:favorites 742
(integer) 1
> SISMEMBER user:123:favorites 299
(integer) 0
```

* 유저 123, 456이 공통으로 좋아하는 책 확인
```shell
> SINTER user:123:favorites user:456:favorites
1) "561"

* set size
```shell
> SCARD user:123:favorites
(integer) 3
```

### Limits
set의 member size는 2^32-1 로 제한된다ㅏ.

### Basic commands
* SADD: member 추가
* SREM: member 제거
* SISMEMBER: set에 member가 있는지 확인 (test membership)
* SINTER: 2개 이상의 set에 공통으로 존재하는 member인지 확인
* SCARD: set size
* SMEMBERS: get all
* SSCAN: iterate member (SSCAN key cursor [MATCH patter] [COUNT count]

### Performance
거의 대부분 O(1)이다. 거대한 set에 대해서 SMEMBERS 커맨드를 사용할 때는 주의해야 한다.  
SMEMBERS는 O(n)이고 모든 set을 single response로 리턴한다. 모든 member를 조회해야 할 때는 SSCAN을 사용하는 것을 권장한다. 

### Alternatives
아주 큰 Set을 저장하려면 메모리를 많이 사용해야 한다.  
만약 메모리 사용에 걱정이 있고 완벽하게 정밀한 데이터가 필요 없다면 [Bloom filter or Cuckoo filter](https://redis.io/docs/stack/bloom/)를 고려해보자

## Hashes
field-value 쌍의 컬렉션으로 이루어진 record type 데이터 구조이다. 기본적인 object 나 counter 등의 그룹을 저장할 수 있다.  

### Examples
* 기본적인 유저 프로파일을 hash로 표현하는 예제
```shell
> HSET user:123 username martina firstName Martina lastName Elisa country GB
(integer) 4
> HGET user:123 username
"martina"
> HGETALL user:123
1) "username"
2) "martina"
3) "firstname"
4) "Martina"
5) "lastName"
6) "Elisa"
7) "country"
8) "GB"
```

* 777 device 카운터를 저장한다. 이 카운터는 ping, issue, request, error 카운트를 저장하는 hash이다.
```shell
> HINCRBY device:777:stats pings 1
(integer) 1
> HINCRBY device:777:stats pings 1
(integer) 2
> HINCRBY device:777:stats pings 1
(integer) 3
> HINCRBY device:777:stats errors 1
(integer) 1
> HINCRBY device:777:stats requests 1
(integer) 1
> HGET device:777:stats pings
"3"
> HMGET device:777:stats requests errors
1) "1"
2) "1"
```

### Basic commands
* HSET: 1개 이상의 field를 추가한다.
* HGET: field의 value를 조회한다.
* HMGET: 한개 이상의 field value를 조회한다.
* HINCRBY: field의 주어진 정수만큼 value를 더한다.
* HRANDFIELD: field의 value N개를 랜덤하게 조회한다.

### Performance
O(N)인 HKEYS, HVALS, HGETALL 등 몇개 명령어 제외하고 모두 O(1)이다.

### Limits
2^32 - 1 개의 field-value 쌍을 저장할 수 있다.

## Sorted sets (Set인데 뭔가 Hash처럼 Key-Value를 가지네)
score 기반으로 정렬해된 set이다. 만약 같은 점수를 가진 경우가 있다면 사전식으로 정렬된다.  
`online game ledaerboard`를 개발하거나 `sliding-window Rate limiter`를 개발할 때 사용될 수 있다.

### Examples
* real-time leaderboard 
```shell
> ZADD leaderboard:455 100 user:1
(integer) 1
> ZADD leaderboard:455 75 user:2
(integer) 1
> ZADD leaderboard:455 101 user:3
(integer) 1
> ZADD leaderboard:455 15 user:4
(intger) 1
> ZADD leaderboard:455 275 user:2
(integer) 0 <<< 랭킹이 올랐다.
```

* 상위 3명 조회
```shell
> ZRANGE leaderboard:455 0 2 REV WITHSCORES
1) "user:2"
2) "275"
3) "user:3"
4) "101"
5) "user:1"
6) "100"
```

* 특정 유저의 랭킹 조회
```shell
> ZREVRANK leaderboard:455 user:2
(integer) 0
```

### Basic commands
* ZADD: member와 스코어를 추가한다. 만약 이미 존재하면 스코어가 업데이트된다.
* ZRANGE: 범위에 해당하는 member를 조회한다.
* ZRANK: 해당 member의 오름차순 랭킹을 조회한다.
* ZREVRANK: ZRANK에서 내림 차순 랭킹을 조회한다.

### Performance
대부분의 명령어가 O(log(n)) 이다. ZRANGE는 O(log(n) + m) 이다.

### Alternatives
데이터에 index와 query가 필요하다면 RedisSearch, RedisJSON을 고려해보자

## Streams
append-only log와 같이 동작하는 자료구조이다. event record용으로 사용할 수 있고 동시 다발적인 event들을 real-time으로 다룰 수 있다.

use case
* Event sourcing (e.g. tracking user actions, clicks)
* Sensor monitoring (e.g. reading from devices)
* Notifications (e.g. storing a record)

각 stream entry 마다 고유 ID가 생성된다. 이 ID를 사용해서 entriy 를 조회하거나 읽고 후속 스트림을 처리할 수 있다. Stream이 무한정 커지는 것을 방지하기 위해서 여러가지 trimming 전략과 consumption 전략을 제공한다.

### Examples
* stream으로부터 온도 읽기
```shell
> XADD temperatures:us-ny:10007 * temp_f 87.2 pressure 29.69 humidity 46
"1658354918398-0"
> XADD temperatures:us-ny:10007 * temp_f 83.1 pressure 29.21 humidity 46.5
"1658354934941-0"
> XADD temperatures:us-ny:10007 * temp_f 81.9 pressure 28.37 humidity 43.7
"1658354957524-0"
```

* 특정 ID부터 2개의 stream entry을 읽는다.
```shell
> XRANGE temperatures:us-ny:10007 1658354934941-0 + COUNT 2
1) 1) "1658354934941-0"
   2) 1) "temp_f"
      2) "83.1"
      3) "pressure"
      4) "29.21"
      5) "humidity"
      6) "46.5"
2) 1) "1658354957524-0"
   2) 1) "temp_f"
      2) "81.9"
      3) "pressure"
      4) "28.37"
      5) "humidity"
      6) "43.7"
```

* stream의 마지막으로부터 새로 생성되는 100개의 stream entry를 읽는다. 만약 stream이 없다면 300ms 동안 BLOCK 된다.

```shell
> XREAD COUNT 100 BLOCK 300 STREAMS temperatures:us-ny:10007 $
(nil)
```

### Basic commands
* XADD: stream에 entry를 추가한다.
* XREAD: 한개 이상의 entry를 조회한다.
* XRANGE: 두 entry ID의 entry들을 조회한다.
* XLEN: stream length

### Performance
ADD == O(1). 
Accessing single entry == O({ID length}). 
    
stream ID는 짧고 고정 길이이다. 이렇게 설계되어서 효율적으로 조회할 수 있게 되었다. 더 알아보려면 [radix tree](https://en.wikipedia.org/wiki/Radix_tree) 구현체를 살펴보자

## Geospatial
좌표를 저장할 수 있는 자료구조이다. radius, bounding box 에 근접한 지점을 탐색하는데에 유용하다.  

### Examples
현재 위치를 기반으로 가까운 전기차 충전소를 찾아주는 모바일 앱을 개발했다고 하자.

* 좌표 저장
```shell
> GEOADD locations:ca -122.27652 37.805186 station:1
(integer) 1
> GEOADD locations:ca -122.2674626 37.8062344 station:2
(integer) 1
> GEOADD locations:ca -122.2469854 37.8104049 station:3
(integer) 1
```

* 5km 반경의 충전소를 탐색한다. 응답값은 이름과 거리이다.
```shell
> GEOSEARCH locations:ca FROMLONLAT -122.2612767 37.7936847 BYRADIUS 5 km WITHDIST
1) 1) "station:1"
   2) "1.8523"
2) 1) "station:2"
   2) "1.4979"
3) 1) "station:3"
   2) "2.2441"
```

### Basic commands
* GEOADD: geospatial index 지역 추가 (lng, lat 순서로)
* GEOSEARCH: redius, boundbox 기준 탐색

## HyperLogLog
set의 cardinality 를 계산하는 데이터 구조이다. 확률론적 데이터 구조로서 공간과 정확도의 트레이드 오프가 있다. 12KB HyperLogLog 구현은 0.81% 의 standard error를 제공한다.  

### Examples
* ADD
```shell
> PFADD members 123
(integer) 1
> PFADD members 500
(integer) 1
> PFADD members 12
(integer) 1
```

* member 수 계산
```shell
> PFCOUNT members
(integer) 3
```

### Basic commands
* PFADD: ADD item
* PFCOUNT: item 숫자 계산
* PFMERGE: 두개 이상의 HyperLogLog를 하나로 병합

### Peformance
상수 시간/공간복잡도: PFADD, PFCOUNT
O(n): PFMERGE

### Limits
2^64 members 까지 계산 가능

## Bitmaps
String을 bit vector 처럼 다룰 수 있게 String 확장 데이터 구조이다. bitwise 명령어도 사용할 수 있다. 

* member가 0 이상의 정수인 set을 표현할 때 유용하다.
* permission 을 표현할 때 유용하다. 예를들어 파일 시스템에서 사용 권한을 지정하는 것.

### Examples
0 ~ 999 까지의 센서가 필드로 존재한다고 가정한다.  
server에 ping 요청을 보낸 센서를 빠르게 탐색해야 하는 상황이라고 하자

* ADD test data
```shell
> SETBIT pings:2024-01-01-00:00 123 1
(integer) 0
```

* 조회
```shell
> GETBIT pings:2024-01-04-00:00 123
1
> GETBIT pings:2024-01-04-00:00 456
0
```

### Basic commands
* SETBIT: set 0 or 1
* GETBIT: return value
* BITOP: bitwise 연산

### Performance
BITOP: O(n)
SETBIT, GETBIT: O(1)

## Bitfields
임의의 길이를 가진 bit에 대해서 set, increment, get integer 연산을 지원하는 데이터 구조이다. unsigned 1-bit ~ signed 63-bit 정수에 대해서 연산을 할 수 있다.  


binary-encoded Redis string 을 사용하여 value를 저장한다.  
atomic read, write, increment 연산을 제공한다.  
counter를 관리하거나 숫자를 관리할 때 좋은 선택지이다.  

### Examples
온라인 게임 안에서의 활동을 추적한다고 가정하자.  
각 플레이어들의 골드와 몬스터 처치 수. 두 개의 메트릭을 유지한다.    

* 새로운 유저가 1000 골드와 튜토리얼을 시작한다.
```shell
> BITFIELD player:1:stats SET u32 #0 1000
1) (integer) 0
```

* 고블린 처치 후 50골드를 얻었고 몬스터 처치 수를 1 증가시킨다.
```shell
> BITFIELD player:1:stats INCRBY u32 #0 50 INCRBY u32 #1 1
1) (integer) 1050
2) (integer) 1
```

* 999 골드로 대거를 구매
```shell
> BITFIELD player:1:stats INCRBY u32 #0 -999
1) (integer) 51
```

* 유저 상태 조회
```shell
> BITFIELD player:1:stats GET u32 #0 GET u32 #1
1) (integer) 51
2) (integer) 1
```

### Basic commands
* BITFIELD: atomically set, increment, read 
* BITFIELD_RO: read-only

### Performance
O(N) number of counter: n