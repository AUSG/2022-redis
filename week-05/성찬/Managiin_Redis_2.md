# Scaling with Redis Cluster
Redis Cluster를 구성해서 수평적 확장을 해보자.

Redis Cluster 구성하면 얻을 수 있는 것
* 자동으로 multiple node로 데이터를 분리할 수 있다.
* 몇개 노드가 죽어도 여전히 redis를 사용할 수 있다.

## Redis Cluster TCP ports
Redis Cluster node는 항상 2개의 TCP 커넥션을 물고 있다. 

* client 에게 제공하는 포트 (default 6379)
* cluster bus port (default 16379)

### cluster bus
cluster bus 는 node - to - node로 소통하는 binary protocol 채널이다.  
cluster bus에서 통신하게 되면 little bandwith, little processing time으로 노드간의 데이터를 교환할 수 있다.  
cluster bus를 통해서 failure detection, configuration update, failover authorization 을 수행한다.  
cluster node들이 key migration을 위해서 client port를 사용하기도 한다.  

## Redis Cluster and Docker
Redis Cluster는 NATted environment는 지원 안한다.  
도커 쓰려면 host networking mode 를 사용해야한다.  

## Redis Cluster data sharding
총 16384 개의 hash slot이 존재한다. 이 slot을 node들이 일정하게 나눠 가진다.  
3개의 노드가 있다면 다음과 같이 슬롯이 분배될 것이다.
* Node A: 0 to 5500.
* Node B: 5501 to 11000.
* Node C: 11001 to 16383.

이런 방식으로 슬롯을 분배하니 노드 숫자의 변경에 아주 유연하다. 그리고 노드 숫자 변경으로 발생하는 down time이 0에 수렴한다.

### hash tags
hash tags를 이용해서 강제로 같은 hash slot에 여러 key를 전달할 수 있다.
`{}` 안에 키를 넣어서 사용하자.
`e.g. user:{123}:profile, user:{123}:account`

## Redis Cluster master-replica model
Redis Cluster는 슬롯마다 replica를 등록해둔다.  
그래서 master가 죽어도 자동으로 promote 되어 모든 slot에 대한 HA를 보장한다.   
(cluster를 구성하기만하면 자동으로 replica를 만들어주는건가?)


## Redis Cluster consistency guarantees
strong consistency를 보장하지 않는다. redis의 asynchronous replication 특성 때문인데 redis가 write하는 동안 아래와 같은 일이 발생할 수 있다. 

* 클라이언트가 마스터 B에 쓰기 작업을 수행한다.
* 마스터 B가 OK를 응답한다.
* 마스터 B가 B1, B2, B3 replica에 쓰기를 전파한다.

위와 같이 B는 B1, B2, B3의 ack 없이 곧바로 client에게 답장을 보내버린다.  
그러니까 위의 시나리오에서는 2 - 3의 사이에서 master가 죽고 promote되는 상황이 발생할 수 있다는 뜻이다. 그러면 client는 쓰기 작업이 완료된 줄 알고 있지만 실제로는 replica들에겐 해당 데이터가 없다.  
  
이 문제는 분산 데이터베이스 환경에서도 전통적으로 발생하는 이슈이며, 해결하기 위해서는 2와 3의 순서를 변경하는 방법이 있다. 이 방법은 성능에 악영향을 미친다. 즉, 트레이드 오프가 있다.  

네트워크 이슈로 인해서 과반수 이상의 replica에 3이 완료되지 않는다면 lose write하게 된다. 일반적으로 promote될 때까지 네트워크 이슈가 지속되면 lose write이고 얼마나 기다려 줄 것인가는 maximum window 를 설정해서 정할 수 있고, node timeout이라고 부른다.

## Redis Cluster configuration parameters
나중에 찾아보자

## Create and use a Redis Cluster
문서의 절차를 보고 만들자.

### Requirements to create a Redis Cluster
redis.conf 파일을 아래와 같이 작성하고 `redis-server ./redis.conf` 명령어로 노드들을 실행시키면 된다.  
```shell
port 7000
cluster-enabled yes
cluster-config-file nodes.conf
cluster-node-timeout 5000
appendonly yes
```
실행시키고나면 각 노드들이 서로를 식별하기 위한 Node ID가 출력된다.
`[82462] 26 Nov 11:56:55.329 * No cluster configuration found, I'm 97a3a64667477371c4479320d683e4c8db5858b1`

# Redis persistence
레디스가 디스크에 데이터를 어떻게 쓰는지 살펴보자자자

**Redis persistence options**
* RDB(Redis Database ㅋㅋㅋㅋㅋㅋ): 일정 interval 마다 point-in-time snapshot 을 남긴다.
* AOF(Append Only File): 서버가 받는 모든 write 요청 마다 로그를 남긴다. 
* no persistence
* RDB + AOF

## RDB
### RDB advantages
* 단일 파일로 data를 표현할 수 있다.
* RDB file은 백업에 용이하다. 예를들어 RDB 파일을 24 시간 기준으로 남기고 RDB snapshot을 30일까지 저장하도록 설정할 수 있다.
* RDB file을 암호화하여 S3에 업로드 하는 등으로 disaster recovery 전략을 세울 수 있다.
* RDB 모드는 disk I/O를 위해 child process를 fork 하여 성능상 이점이 있다.
* AOF 에 비해서 빅 데이터 셋에도 빠른 재시작을 지원한다.
* replica를 사용하고 있다면 partial resynchronization after restarts and failover 를 지원한다. 

### RDB disadvantages
* Redis가 멈춘 상황에서 data loss를 줄이고 싶다면 좋은 선택지는 아니다. RDB는 일정 시간마다 disk write를 수행하는데 그 시간 전에 Redis가 죽을 수 있다.
* RDB는 데이터를 디스크에 쓰기 위해서 fork()를 수행한다. 만약 데이터가 크다면 fork() 는 시간을 많이 사용하게 된다.

## AOF
### AOF advantages
* AOF는 내구성이 더 좋다. 
  * fsync 에 대한 여러가지 정책을 선택할 수 있다. (fsync every second, every query)
  * default인 fsync every second 도 쓰기 성능이 훌륭하다.
  * 쓰기는 백그라운드 프로세스가 수행하고, 백그라운드 프로세스가 없을 때만 메인 스레드가 쓰기를 시도하므로 1초간의 lose write가 발생한다.
* AOF 는 append-only 이므로 비정상적 종료에도 검색, 데이터 훼손 문제가 없다.
  * 쓰다가 갑자기 종료되어도 redis-check-aof tool이 해당 문제를 쉽게 고쳐준다.
* Redis는 AOF 파일이 너무 커지면 rewrite를 활성화한다. 
  * rewrite는 새로운 파일 생성과, 기존 파일 둘 다 완전히 안전하게 수행된다.
* AOF 는 파싱하기 쉬운 포맷으로 모든 operation에 대한 로그가 적재된다.

### AOF disadvantages
* 