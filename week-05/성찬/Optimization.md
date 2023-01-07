# Optimization
## Redis benchmark
redis-benchmark는 redis에 포함된 유틸리티 툴이다. 이걸로 N 개의 클라이언트가 동시에 M 개의 쿼리를 날리는 상황을 시뮬레이션 할 수 있다.
레디스 서버를 실행시킨 후 `redis-benchmark -q -n 100000` 명령어로 벤치마크를 수행할 수 있다.  

### Running only a subset of the tests
항상 모든 테스트셋을 모두 실행시킬 필요는 없고 subset만 실행할 수도 있다.
```shell
$ redis-benchmark -t set, lpush -n 1000000 -q
SET: 74239.05 requests per second
LPUSH: 79239.30 requests per second


$ redis-benchmark -n 100000 -q script load "redis.call('set', 'foo', 'bar')"
script load redis.call('set', 'foo', 'bar'): 69881.20 requests per second
```

### Selecting the size of the key space
기본적으로 단일 키에 대해서만 벤치마크한다. 현실 세계와 유사한 워크로드를 구성하기 위해서 large key space, cache misse 상황도 벤치마크 할 수 있다.

```shell
$ redis-cli flushall
OK

$ redis-benchmark -t set -r 1000000 -n 1000000
=== SET ===
  100000 requets completed in 13.86 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1
  
  99.76% `<=` 1 milliseconds
  99.98% `<=` 2 milliseconds
  100.00% `<=` 3 milliseconds
  100.00% `<=` 3 milliseconds
  72144.87 requests per second
  
$ redis-clit dbsize
(integer) 99993
```

### Using pipelining
파이프라인을 사용할 때의 벤치마크도 가능하다.
```shell
$ redis-benchmark -n 10000000 -t set,get -P 16 -q
SET: 403063.28 requests per second
GET: 508388.41 requests per second
```


### Pitfails and misconceptions
벤치마크할 때 유념해야 하는 것은 같은 것을 비교해야한다는 것이다.
* 임베디드 저장소와 원격 저장소를 비교하는 짓은 하지 말자
* Redis는 요청에 대한 acknowledment를 반환한다. 그렇지 않은 DB도 있다. 유념하자.
* 하나의 커넥션으로 요청을 보내면 레디스에 부하를 걸기 어렵다. 네트워크 지연시간 때문이다. 레디스를 테스트하고 싶다면 멀티 커넥션을 맺고 부하를 만들자
* 레디스에 영속성 옵션이 있지만 RDB들과 비교하려면 AOF 를 활성화하고 fsync policy 를 잘 설정해야한다.
* 레디스는 싱글 스레드이기 때문에 멀티 스레드를 사용하는 저장소와 비교하는 것은 불공정하다.

### Factors impacting Redis performance
* 네트워크 대역폭과 지연시간이 레디스 성능에 영향을 미친다. 네트워크 대역폭은 대부분의 경우 CPU가 과부하되기 전에 이미 가득판다.
* CPU도 중요한 요인이다. 싱글 스레드 환경에서는 아직까진 인텔 CPU가 좋다.
* RAM 메모리 대역폭도 중요하다. 10KB 이상의 큰 정보를 주고 받을 때 특히 영향을 많이 준다.
* 물리장비에 직접 띄우는 것보다 VM 위에 띄우는게 더 느리다.


## Redis CPU profiling
레디스에 병목이 생겼을 때 이게 CPU 때문인지 확인하려면 프로파일링을 해보자.

### Build Prerequisites
일단 레디스를 직접 빌드해야한다.
`$ make REDIS_CFLAGS="-g -fno-omit-frame-pointer`
-g 로 디버그 모드를 켜고 -fno-omit-frame-pointer 로 프레임 포인터 레지스터를 제공하자.

## Latency diagnosis
slow response의 원인을 찾아보자.

### checklist
1. Redis Slow Log feature 를 사용해서 slow command가 수행되고 있는지 확인해라
2. EC2 유저라면 HVM 머신을 사용해라. 그외에는 fork()가 너무 느리다.
3. Transparent huge pages를 비활성화 해라. `echo never > /sys/kernel/mm/transparent_hugeparent/enabled` 그리고 재시작
4. VM 에 올라가있다면 `./redis-cli -intrinsic-latency 100` 명령어를 사용해서 지연시간을 최소화하자
5. Latency monitor 를 활성화해서 관측하자.

위의 체크리스트로도 지연시간 해결이 안된다면 문서를 다 읽어봐라.

## Latency monitoring

## Memory optimization
메모리 최적화 전략
### Special encoding of small aggregate data types
2.2 버전부터 데이터 타입들이 메모리를 조금만 사용할 수 있도록 최적화를 하고 있다. 인코딩을 통해 평균 5배, 최대 10배 절약중이다.  
물론 인코딩은 CPU/Memory 의 트레이드 오프이다. 아래 설정을 조절해서 잘 써보자
```shell
hash-max-ziplist-entries 512
hash-max-ziplist-value 64
zset-max-ziplist-entries 128
zset-max-ziplist-value 64
set-max-intset-entries 512
```

### Using 32 bit instances
레디스를 32비트 바이너리 파일로 빌드해라. 키에 할당되는 메모리가 매우 적어질 것이다.  
32비트 저장된 AOF, RDB 파일을 64비트로 바꿔도 전혀 문제 없다.  

### Bit and byte level oprations
bit, byte level operation을 사용하면 array random access를 해결할 수 있다.  
근데 솔직히 비트 연산자 이런거 앵간하면 쓰지말자

### Use hashses when possible
작은 hashes는 인코딩을 통해서 아주 작은 공간만 사용한다. 앵간하면 hashes를 사용하자  
예를들어서 name, surname, email, password 를 각각의 키로 저장하지말고 단일 키로 저장하자.  

### Using hashes to abstract a very memory efficient plain key-value store on top of Redis

### Memory allocation
레디스의 메모리 관리법 몇가지..
* 키가 지워졌다고 바로 OS에게 메모리를 반납하지 않는다.
  * 사실 레디스의 구현 때문은 아니고 malloc() 이 그렇게 동작하기 때문이다.
  * 위의 사실이 의미하는게 모냐면 메모리를 넉넉하게 준비해놓으라는 소리다. 만약에 애플리케이션이 아주 가끔 10GB를 사용하고 평소에는 5GB를 사용하더라도 10GB를 준비하라는 소리다.  
* allocator는 똑똑해서 최근에 free된 메모리를 재사용한다. 만약에 5GB 중 2GB를 free 했다면 (free 해도 OS에게 반납은 안함) 새로 추가되는 키는 2GB에 할당된다.  
* 위의 사실 때문에 fragmentation ratio를 신뢰할 수 없다.
  * peak usage가 currently usage 보다 훨씬 크다.
  * fragmentation은 physical memory actually used / currently usage 로 계산된다.


maxmemory 설정 안되어있으면 메모리 계속 할당하면서 메모리를 다 먹어치울꺼다.
