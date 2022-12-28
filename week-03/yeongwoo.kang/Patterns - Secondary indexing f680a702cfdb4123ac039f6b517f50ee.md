# Patterns - Secondary indexing

생성일: 2022년 12월 18일 오후 4:24

- 값이 복잡한 데이터 구조일 수 있기 때문에 Redis는 정확히 key-value 저장소가 아니다. 그러나 외부 key-value shell이 있다. API 수준에서 데이터는 키 이름으로 지정된다.
- Redis는 데이터 구조 서버이므로 복합(다중 열) 인덱스를 포함하여 다양한 종류의 보조 인덱스를 생성하기 위해 인덱싱에 Redis의 기능을 사용할 수 있다.
- 인덱스를 만들기 위한 데이터 구조
    - ID 또는 기타 숫자 필드로 보조 인덱스를 생성하기 위한 정렬된 세트
    - 고급 보조 인덱스, 복합 인덱스 및 그래프 순회 인덱스를 만들기 위한 사전식 범위가 있는 정렬된 집합
    - 임의 인덱스 생성을 위한 세트
    - 간단한 반복 가능한 인덱스 및 마지막 N 항목 인덱스를 만들기 위한 목록
- 캐싱 시나리오에서 실행하기 위해 어떤 형태의 인덱싱이 필요한 일반 쿼리의 속도를 높이기 위해 인덱싱된 데이터를 Redis에 저장해야 하는 명시적인 필요성이 있다

# Simple numerical indexes with sorted sets

- Redis로 생성할 수 있는 가장 간단한 보조 인덱스는 각 요소의 점수인 부동 소수점 숫자로 정렬된 요소 집합을 나타내는 데이터 구조인 정렬된 집합 데이터 유형을 사용하는 것이다. 요소는 가장 작은 점수에서 가장 높은 점수 순으로 정렬된다.
- 점수는 배정밀도 부동 소수점이므로 바닐라 정렬 세트로 빌드할 수 있는 인덱스는 인덱싱 필드가 주어진 범위 내의 숫자인 항목으로 제한된다.
- 이러한 종류의 색인을 작성하는 두 가지 명령은 BYSCORE 인수가 있는 ZADD 및 ZRANGE로, 각각 항목을 추가하고 지정된 범위 내에서 항목을 검색한다.
- 예: 정렬된 집합에 요소를 추가하여 나이별로 사람 이름 집합을 인덱싱할 수 있다. 요소는 사람의 이름이 되고 점수는 나이가 된다.
    
    ```bash
    ZADD myindex 25 Manuel
    ZADD myindex 18 Anna
    ZADD myindex 35 Jon
    ZADD myindex 67 Helen
    
    # 20세에서 40세 사이의 모든 사람을 검색하려면 다음 명령을 사용할 수 있다.
    ZRANGE myindex 20 40 BYSCORE
    1) "Manuel"
    2) "Jon"
    ```
    
- ZRANGE의 WITHSCORES 옵션을 사용하면 반환된 요소와 관련된 점수를 얻을 수도 있다.
- ZCOUNT 명령은 실제로 요소를 가져오지 않고 주어진 범위 내의 요소 수를 검색하기 위해 사용할 수 있으며, 이는 특히 범위의 크기에 관계없이 작업이 로그 시간에 실행된다는 사실을 고려할 때 유용하다. (범위는 포괄적이거나 배타적일 수 있다.)
- 참고: BYSCORE 및 REV 인수와 함께 ZRANGE를 사용하면 역순으로 범위를 쿼리할 수 있다. 이는 데이터가 지정된 방향(오름차순 또는 내림차순)으로 인덱싱될 때 유용하지만 다른 방법으로 정보를 검색하려는 경우에 유용하다.

## Using objects IDs as associated values

- 다른 곳에 저장된 개체의 일부 필드를 인덱싱할 수 있다.
- 인덱스된 필드와 관련된 데이터를 저장하기 위해 정렬된 집합 값을 직접 사용하는 대신 개체의 ID만 저장할 수 있다.
- 예: 사용자를 나타내는 Redis 해시가 있을 수 있다. 각 사용자는 ID로 직접 액세스할 수 있는 단일 키로 표시된다.
    
    ```bash
    HMSET user:1 id 1 username antirez ctime 1444809424 age 38
    HMSET user:2 id 2 username maria ctime 1444808132 age 42
    HMSET user:3 id 3 username jballard ctime 1443246218 age 33
    
    # 나이별로 사용자를 쿼리하기 위해 색인을 생성하려면 다음을 수행할 수 있다.
    ZADD user.age.index 38 1
    ZADD user.age.index 42 2
    ZADD user.age.index 33 3
    ```
    
- 이번에는 정렬된 집합의 점수와 관련된 값이 개체의 ID다.
    - 따라서 BYSCORE 인수를 사용하여 ZRANGE로 인덱스를 쿼리하면 HGETALL 또는 유사한 명령을 사용하여 필요한 정보도 검색해야 한다.
    - 장점: 색인된 필드를 변경하지 않는 한 색인을 건드리지 않고도 개체가 변경될 수 있다.
- 몇 가지 예외를 제외하고 일반적으로 더 그럴듯한 디자인이기 때문에 앞으로 거의 항상 인덱스와 관련된 값으로 ID를 사용한다.

## Updating simple sorted set indexes

- 시간이 지남에 따라 변경되는 항목을 색인화 하는 경우 ZADD 명령을 통해 다른 점수와 같은 값을 가진 요소를 다시 추가하면 단순히 점수를 업데이트하고 요소를 올바른 위치로 이동하기 때문에 간단한 인덱스 업데이트를 매우 간단한 작업으로 만든다.
    
    ```bash
    # user:1의 나이가 39세가 된 경우 사용자를 나타내는 해시와 인덱스의 데이터를 업데이트하는 명령
    HSET user:1 age 39
    ZADD user.age.index 39 1
    ```
    
- 두 필드가 모두 업데이트되었는지 확인하기 위해서는 작업이 MULTI/EXEC 트랜잭션으로 래핑될 수 있다.

## ****Turning multi dimensional data into linear data****

- 정렬된 집합으로 생성된 인덱스는 단일 숫자 값만 인덱싱할 수 있다.
- 선형 방식으로 다차원을 효율적으로 표현할 수 있는 경우 인덱싱을 위해 간단한 정렬 집합을 사용하는 것이 가능하다.
    - 지리 인덱싱 API: 위도/경도 → 해시값

# Lexicographical indexes(사전 색인)

- ZRANGE 및 ZLEXCOUNT 같은 명령은 모든 요소의 점수가 동일한 정렬된 집합과 함께 사용 된다고 가정할 때 사전적 방식으로 범위를 쿼리하고 계산할 수 있다. 이를 이용해 인덱스 구현 가능
    
    ```bash
    # 점수가 같아야하기 때문에 score는 항상 0점
    ZADD myindex 0 baaa
    ZADD myindex 0 abbb
    ZADD myindex 0 aaaa
    ZADD myindex 0 bbbb
    
    # 정렬된 세트에서 모든 요소를 가져오면 사전순으로 정렬됨을 확인 가능
    ZRANGE myindex 0 -1
    1) "aaaa"
    2) "abbb"
    3) "baaa"
    4) "bbbb"
    
    # 이제 범위 쿼리를 수행하기 위해 BYLEX 인수와 함께 ZRANGE를 사용할 수 있음
    ZRANGE myindex [a (b BYLEX
    1) "aaaa"
    2) "abbb"
    
    # 무한 음수 문자열과 무항 양수 문자열을 사용한 쿼리
    ZRANGE myindex [b + BYLEX
    1) "baaa"
    2) "bbbb"
    ```
    

## A first example: completion

- 사용자가 검색을 하려고 검색어를 입력할 때, 검색어로 시작하는 단어들에 대해서 색인결과를 찾을 수 있음. 너무 많은 값이 나올 경우 limit를 걸 수도 있음
    
    ```bash
    # 사용자가 입력한 banana를 인덱스에 추가
    ZADD myindex 0 banana
    # 사용자가 입력한 bit와 이 단어로 시작되는 단어들을 찾는다
    ZRANGE myindex "[bit" "[bit\xff" BYLEX
    ```
    

## Adding frequency into the mix

- 위의 접근 방식에서 빈도에 따라 문자열을 완성하려고 한다.
- 인기있는 검색어가 더 높은 확률로 제안하려고 한다.
- 더 이상 인기가 없는 검색을 제거하여 빈도에 따라 달라지는 동시에 향후 입력에 자동으로 적응하는 무언가를 구현하기 위해 매우 간단한 스트리밍 알고리즘을 사용할 수도 있다.
    
    ```bash
    # banana에 1을 붙인다. 1은 빈도다.
    ZADD myindex 0 banana:1
    
    # 검색어가 인덱스에 이미 존재하는 경우를 찾는다
    ZRANGE myindex "[banana:" + BYLEX LIMIT 0 1
    1) "banana:1"
    
    # 존재하는 경우 바나나의 단일 항목 반환. frequency를 증가시키고 다음 두 명령을 보낸다.
    ZREM myindex 0 banana:1
    ZADD myindex 0 banana:2
    ```
    
- 동시 업데이트가 있을 수 있기 때문에 lua스크립트를 활용해야 원자적으로 적용 가능
    
    ```bash
    ZRANGE myindex "[banana:" + BYLEX LIMIT 0 10
    1) "banana:123"
    2) "banaooo:1"
    3) "banned user:49"
    4) "banning:89"
    ```
    
    - 위와같은 결과가 나왔을 경우, banaooo를 사용하지 않지만 쿼리를 통해 조회되었으므로 1을 감소시켜야한다. 0에 다다르면 삭제시킨다.
    - 이를 통해 자동적으로 인기 검색어를 유지시킬 수 있다.

## Normalizing strings for case and accents

> 대소문자와 악센트를 위한 문자열 정규화
> 
- 사용자가 실제로 검색하는 문자열을 정규화 시킨다.
    - Banana, BANANA, Ba’nana → banana
- 하지만 실제 원래 항목을 사용자에게 제공하고 싶을 수 있다.
    - term:frequency → normalized:frequency:original 로 저장한다.
        
        ```bash
        ZADD myindex 0 banana:273:Banana
        ```
        

## ****Adding auxiliary information in the index****

> 색인에 보조정보 추가
> 
- 정렬된 집합을 사용하는 경우, 각 개체에 대해 인덱스로 사용하는 점수와 관련 값. 이렇게 두 가지 속성이 있다.
- 인덱싱 키에 모든 종류의 관련 값을 추가할 수 있다. 간단한 key-value 저장소를 구현하기 위해 사전식 색인을 사용하려면 항목을 `key:value`로 저장하면 된다
    
    ```bash
    # 저장하기
    ZADD myindex 0 mykey:myvalue
    
    # 찾기
    ZRANGE myindex [mykey: + BYLEX LIMIT 0 1
    1) "mykey:myvalue"
    ```
    
- 콜론 뒤의 부분을 추출하여 값을 검색한다. 하지만 이경우 콜론 문자는 키 자체의 일부일 수 있으므로 추가한 키와 충돌이 될 수 있다.
- Redis의 사전식 범위는 이진 안전이므로 모든 바이트 또는 모든 바이트 시퀀스를 사용할 수 있다. 그러나 신뢰할 수 없는 사용자 입력을 받는 경우 구분 기호가 키의 일부가 되지 않도록 보장하기 위해 어떤 형태의 이스케이프 문자를 사용하는 것이 좋다.
- 예: 2개의 null byte를 구분 기호 “\0\0”로 사용하는 경우 항상 null 바이트를 문자열에서 2바이트 시퀀스로 이스케이프할 수 있다.

## Numerical padding

- 임의 정밀도 숫자의 인덱싱을 수행하기 위해 숫자 앞에 0을 추가하면 숫자를 문자열로 비교하면 숫자값으로 정렬된다.

```bash
ZADD myindex 0 00324823481:foo
ZADD myindex 0 12838349234:bar
ZADD myindex 0 00000000111:zap

ZRANGE myindex 0 -1
1) "00000000111:zap"
2) "00324823481:foo"
3) "12838349234:bar"
```

- prefix 0, postfix 0으로 소수부분을 채우면 모든 정밀도의 부동 소수점 숫자도 사용할 수 있다.

```bash
		01000000000000.11000000000000
    01000000000000.02200000000000
    00000002121241.34893482930000
    00999999999999.00000000000000
```

## ****Using numbers in binary form****

> 이진 형식의 숫자 사용
> 
- 10진수로 저장하면 메모리를 굉장히 많이 잡아 먹는다.
- 저장되기 전 big edian형식으로 저장할 수 있다. 이러면 효과적으로 정렬된다.
- 하지만 디버깅이 어렵고 구문 분석 및 내보내기가 어려워진다.

# Composite indexes

> 복합 인덱스
> 
- 여러개의 인덱스를 한번에 사용하고 싶을 때가 있음
- 예: 어떤 방의 상품 번호와 가격을 인덱싱으로 저장
    
    ```bash
    # room:price:product_id
    ZADD myindex 0 0056:0028.44:90
    ZADD myindex 0 0034:0011.00:832
    
    # 56번방의 10~30달러의 물품을 찾는 쿼리
    ZRANGE myindex [0056:0010.00 [0056:0030.00 BYLEX
    ```
    

# Updating lexicographical indexes

> 사전식 인덱스 업데이트
> 
- 사전 색인값은 저장한 내용을 다시 작성하기가 어렵고 느리다.
- 메모리를 사용해서 개체 ID를 현재 인덱스 값에 매핑하는 해시를 인덱스를 나타내는 정렬된 세트와 함께 가져감으로써 인덱스 처리를 단순화할 수 있다.

```bash
# 색인을 생성할 때 해시에도 추가된다.
MULTI
ZADD myindex 0 0056:0028.44:90
HSET index.content 90 0056:0028.44:90
EXEC
```

- 이는 항상 필요한 것은 아니지만 인덱스 업데이트 작업을 단순화한다.
- 개체 ID 90에 대해 인덱싱한 정보를 제거하려면 개체의 현재 필드 값에 관계없이 개체 ID별로 해시 값을 검색하고 정렬된 집합 보기에서 ZREM해야한다.

# Representing and querying graphs using a hexastore

> 헥사스토어를 사용한 그래프 표현 및 쿼리
> 
- 복합 인덱스와 Hexastore 구조를 사용해 그래프를 표현하는데 편리하다.
- Hexastore: Subject, predicate, object로 구성된 객체간의 관계에 대해 표현한다.
    
    ```bash
    # 주어 - 서술어 - 목적어
    antirez is-friend-of matteocollina
    ```
    
- 이 관계를 나타내기 위해 사전 색인에 다음 요소를 저장할 수 있다.
    
    ```bash
    # spo를 prefix로 붙여서 주어-서술어-목적어 관계를 명시함
    ZADD myindex 0 spo:antirez:is-friend-of:matteocollina
    
    # 동일한 관계에 대해 다른 순서로 나타낼 수 있음
    ZADD myindex 0 sop:antirez:matteocollina:is-friend-of
    ZADD myindex 0 ops:matteocollina:is-friend-of:antirez
    ZADD myindex 0 osp:matteocollina:antirez:is-friend-of
    ZADD myindex 0 pso:is-friend-of:antirez:matteocollina
    ZADD myindex 0 pos:is-friend-of:matteocollina:antirez
    
    # 안티레즈가 친구인 사람들은 누구인가?
    ZRANGE myindex "[spo:antirez:is-friend-of:" "[spo:antirez:is-friend-of:\xff" BYLEX
    1) "spo:antirez:is-friend-of:matteocollina"
    2) "spo:antirez:is-friend-of:wonderwoman"
    3) "spo:antirez:is-friend-of:spiderman"
    
    # 또는 첫 번째가 주어이고 두 번째가 목적어인 안티레즈와 마테오콜리나의 모든 관계는 무엇인가?
    ZRANGE myindex "[sop:antirez:matteocollina:" "[sop:antirez:matteocollina:\xff" BYLEX
    1) "sop:antirez:matteocollina:is-friend-of"
    2) "sop:antirez:matteocollina:was-at-conference-with"
    3) "sop:antirez:matteocollina:talked-with"
    ```
    

# Multi dimensional indexes

> 다중 차원 인덱스
> 
- 나중에 알아보자…