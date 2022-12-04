# Week 1

## About

- [사용 기업 사례](https://redis.io/docs/about/users/)
    - 트위터
    - 깃허브
    - 스냅챗
    - Craigslist
    - 스택오버플로우

- 배포판으로는 **Linux를 사용하는것이 좋음**
    - ANSI C로 만들어졌고 POSIX 시스템(Linux, *BSD, OSX)에서 디펜던시 없이 작동
    - Windows에는 official support가 없음

## Data Types

### 개요
- `Strings` : byte sequence, 기본 자료형
- `Lists` : 삽입 순서대로 sorting되는 string 리스트
- `Sets` : unordered / unique collection. 해시셋과 비슷. 삽입,삭제, 값 확인에 O(1)
- `Sorted Sets` : 각 string의 associated socre에 따라 순서를 유지하는 Sets
- `Hashes` : 필드(키)-값으로 이루어진 형태. Dictionary 혹은 해시맵과 비슷
- `Streams` : append-only log형태. 이벤트 발생 순으로 이벤트를 기록하고, 처리를 위해 이벤트들을 syndicate(연합?)함.
- `Geospactioal Indexes` : 주어진 geographic radius 혹은 bounding bo에 속하는 location을 찾을때 유용
- `Bitmaps` : string에 대해 bitwise operation 수행 가능
- `Bitfields` : 효율으로 multiple counter를 String으로 encode할 때 사용. atomic get/set/increment operration을 제공하고, overflow를 처리하는 정책을 지원
- `HyperLogLog` : 커다란 set의 cardinality(해당하는 element의 갯수)에 대한 확률적 추정 제공.

[Redis Stack](https://redis.io/docs/stack/)을 사용하면 아래 지원도 가능.
- 쿼리용 Json docs
- Hashes와 Json docs간의 쿼리
- 시계열 자료구조에 대한 지원(저장 및 쿼리)
- 그래프 데이터 모델에 대한 지원


### Keys
- Redis Key는 binary safe하여 Binary Sequence면 어떤것이라도 키로 사용할 수 있음. string이든 JPEG 파일 content든 빈 문자열이든.
- Key값이 너무 큰것은 메모리/bandwidth/조회 효율상 좋지 않음. 큰 값이 있다면 해싱해서 크기를 줄여서 사용할것
- 최대 키 크기는 512MB


#### Expiration
- TTL은 s, ms 단위로 설정 가능
- 만료 시간 확인은 1 ms마다 수행
- 만료 정보는 디스크 복제/지속

---
### String
- set 명령 시 key duplicated, not found(의역) 옵션 지정 가능
    - setnx : 키가 없을때에만 값 저장
- mget/mset : atomic하게 여러 값을 조회/설정
    - mget 사용시 값의 배열 반환
- getset : 키를 새 값으로 설정하고 이전 값을 결과로 반환. 이 과정을 모두 atomic하게 수행.
    - 방문자 기록(조횟수) 늘릴때 유용

#### Performance
- getrange, setrange 명령은 O(n)이 될수있으므로 주의할것.
    - 문자열 substring 반환 / 변경

---
### Lists
- stack/queue 구현에 주로 사용
    - 백그라운드 워커시스템을 위한 큐 관리 용도
- max length는 2^32-1(약 42억9천)
- redis List는 array가 아닌 linked list
    - 따라서 index access가 O(1)이 아님
    - 대규모의 collection에서 중간 요소에 access하고싶다면 sorted set을 사용할것
- indeterminant한 event 요소 저장시에는 list가 아니라 redis stream 사용할것

#### 사용사례
- 소셜 네트워크에서 latest updates를 가져오기(twitter)
    - 최근 n개의 무언가
- consumer/producer

#### Performance
- consumer/producer에서 RPOP/LPOP 사용할 때 null일수 있으므로 주기적으로 요청해주어야하는데, 이런 polling 방식은 오버헤드
    - BRPOP, BLPOP사용해서 지정시간동안 큐에 컨텐츠가 차면 가져오도록 할수 있음
    - 지정 시간 지나면 null반환
- lindex, linsert, lset은 O(n)이므로 주의할것(linked list의 특성)


---
### Hashes

- 필드 갯수는 메모리가 허용하는 한 제한이 없음
- hincrby 사용하면 해시 내의 개별 필드에서도 atomic operation 수행 가능

#### Performance
- hkeys, hvals, hgetall같은 특수한 명령어만 O(n)이고 나머지는 다 O(1)

---
#### Sorted Sets

- Set과 Hash를 섞어놓은것에 가까움
    - 내부적으로는 skip list와 hash table이 섞여있는 구조, 삽입에 O(log N)
- `score`라는 floating point value에 모든 element가 매핑되어있음
    - 이 score 기준 내림차순 정렬
    - score가 같을경우 string값이 사전식으로 더 크면 큰 값
    - score 값은 임의 지정 가능
- 사전순 호출 사용 가능(>=V2.8)
- 사용사례
    - 랭킹 시스템 구현
    - rate limiter로 과도한 api 요청 수 제한

----
### Bitmaps

- 단일 비트에 대한 연산 / 비트 그룹에 대한 연산 둘다 가능
- 정보 저장시 메모리 효율이 좋음
    - ex) autoincrement id로 저장되는 시스템에서 1bit당 한 사람의 bool값을 저장할 수 있음.(뉴스레터 수신여부 같은 것 저장 시에 유용)
- 키가 너무 커지는 것을 방지하기위해 키당 비트 수를 제한해서 modulo로 샤딩
- 사용 사례
    - longest streak of daily visits(출석체크 같은)
    - 파일시스템같은 객체 권한 관리
    - 온라인 게임에서 맵 탐색

#### Performance
- 비트문자열 비교하는 BITOP은 O(n), 나머지는 다 O(1)

---
### HyperLogLog

- set 카디널리티(고유한 값 크기)에 대한 근사 추정
    - 고유 갯수 추정은 이전의 모든 값을 기억해야 하므로 memory 효율이 좋지 않으나.. 이 방식은 최대 12KB만 사용하고, 약 1%의 오차율
- 사용사례
    - 트래픽 히트맵 구현
- 최대 2^64개의 크기까지만 추정 가능

#### Performance
- PFADD / PFCOUNT는 둘다 Constant time

---
### Stream

- append only 로그
- 각 스트림마다 unique id 생성
- 사용사례
    - 이벤트 소싱
    - 사용자별 notification 기록 저장

#### Performance
- 추가시 O(1), access 시에 O(n)이고 n은 ID 길이. 근데 ID 길이는 일반적으로 짧게 가져가므로 효율적
    - [Radix 트리](https://en.wikipedia.org/wiki/Radix_tree) 사용하기 때문

---
### Geospatial

- 현재 위/경도 기준으로 반경 혹은 bounding box안에 있는 모든 데이터 좌표 검색가능
- 사용사례
    - 내 반경 5KM 자동차 충전소 목록 검색

---
### Bitfield
??

---
## Further reading
- [redis SCAN의 동작](https://tech.kakao.com/2016/03/11/redis-scan/)
    - 왜 scan이 SMEMBER, KEYS등 보다 효율적일까?
- [확률적 자료구조를 이용한 추정 - 유일한 원소 개수(Cardinality) 추정과 HyperLogLog](https://d2.naver.com/helloworld/711301)
    - HyperLogLog 작동 원리
    - [stackoverflow 글도 참고](https://stackoverflow.com/questions/12327004/how-does-the-hyperloglog-algorithm-work)
