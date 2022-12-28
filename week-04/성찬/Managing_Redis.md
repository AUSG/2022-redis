# Managin Redis (Week 4)

## Redis administration
레디스 설정과 운영을 훈수하는 페이지

### Setup tips
* OSX, FreeBSD, OpenBSD 시스템에서 테스트 되었는데 linux에서 제일 성능이 좋았다.
* linux kernel overcommit memory 를 1로 설정하자. `cat "vm.overcommit_memory = 1" >> /etc/sysctl.conf && sysctl vm.overcommit_memory=1` 
  * 유휴 자원이 있을 때 지정된 메모리 또는 cpu보다 더 많이 사용하는 것  ([overcommit article](https://www.linuxembedded.fr/2020/01/overcommit-memory-in-linux))
* linux kernel 의 Transparent Huge Pages(THP)가 Redis memory 사용량과 지연시간에 영향을 미치지 않을 것을 확실히 하기 위해서 `echo never > /sys/kernrl/mm/transparent_hugepage/enabled` 명령어를 입력하자
  * fixed allocation 말고 클 때는 큰걸(2MB) 주는걸 THP라고 부른다.

### Memory
* swap memory를 memory size 만큼 설정해라. 안그러면 갑자기 너무 많은 메모리르 사용하게 될 때 OOM 나거나 OOM Killer에 의해서 레디스 프로세스 죽는다.
  * swap memory 설정해가지고 latency 증가하는걸 모니터링하고 대응하자
* maxmemory 를 명시적으로 설정하고 메모리가 사용량이 많으면 report를 받자
* 큰 파일을 디스크에 쓸 때 Redis가 원래 필요할 메모리의 2배의 메모리 사용할 수 있다.
* `LATENCY DOCTOR` , `MEMORY DOCTOR` 명령어를 사용해서 트러블 슈팅에 도움을 받자

### Replication
* Redis 가 사용중인 메모리에 비교하여 non-trivial replication backlog를 구축하자. backlog가 master-replication sync를 도와줄꺼야
* replication 사용하면 persistence disable이어도 RDB save처럼 동작할꺼다. 만약 disk usage가 없으면 enable diskless replication 하자.
* replication 사용하고 있으면 master에 enable persistence 또는 not automatically restart on crashes 설정해라. 
  * replica는 master와 동일한 상태를 유지하려고 값을 계속 복사할껀데 master가 재시작되면 데이터가 empty가 된다. 그럼 replica도 wiped as well. bb

### Security
* Redis는 기본적으로 인증이 필요 없다. 그러면 안되니까 인증은 알아서 잘 설정하자. [redis security](https://redis.io/docs/management/security/) 문서를 보는 것도 괜찮을듯

### Running Redis on EC2
* Hardware VM based instance 사용하자. ParaVirtual based는 사용하지 말자.
* old instance family 사용하지 말자. m3.medium > m1.medium


### Upgrading or restarting a Redis instance without downtime
레디스는 log-running process 로 설계되었다. `CONFIG SET {command}` 명령어를 사용하면 재시작 없이 대부분의 설정을 반영할 수 있다.

## High availability with Redis Sentinel
non-clustered Redis의 High availability

**Redis Sentinel의 굵직한 기능**
* Monitoring
* Notification
* Automatic failover (promote replica to master)
* Configuration provider

### Sentinel as a distributed system
Sentinel은 분산 시스템이다. 얘는 다수의 Sentinel process와 협동하여 동작하도록 하는 configuration 으로 실행된다.  
멀티 센티널 프로세스 방식이 가져오는 이점은 다음과 같다.
1. failure detection은 여러 프로세스가 master에 접근할 수 없다는 사실에 동의 할 때만 동작한다. 이러면 false positive 가능성이 낮아진다.
2. 모든 센티널 프로세스가 실행중이 아니여도 동작한다. 

### Sentinel quick start

#### Running Sentinel
redis-sentinal: `redis-sentinel /path/to/sentinel.conf`  
redis-server: `redis-server /path/to/sentinel.conf --sentinel`  
반드시 conf 파일을 수정해야한다.

#### Fundamental things to know about Sentinel before deploying
1. 최소 3개의 Sentinel instance 는 셋팅해야 쓸만하다.
2. 인스턴스들은 격리된 공간에서 실행되어야 한다. 독립적으로 fail 하도록.
3. redis async replication 이기 때문에 write를 보장하지 않는다.
4. Sentinel support 라는 클라이언트를 사용하자. 다른것도 있긴 함.
5. 개발환경에서 계속 테스트해봐라 (라고 하는데 어떻게 테스트하는데?)
6. Sentinel, Docker, NAT 같은거 사용할 때 만약 Docker는 port remapping으로 auto discovery 한다.

#### Configuring Sentinel
**typical minimal configuration example**
```shell
sentinel monitor mymaster 127.0.0.1 6379 2
sentinel down-after-milliseconds mymaster 60000
sentinel failover-timeout mymaster 180000
sentinel parallel-syncs mymaster 1

sentinel monitor resque 192.168.1.3 6380 4
sentinel down-after-milliseconds resque 10000
sentinel failover-timeout resque 180000
sentinel parallel-syncs resque 5
```

master에만 monitor 설정하면 된다. replica는 알아서 식별된다. 그리고 configuration은 replica promote 될 떄 마다 재작성된다.


## Replication
문서의 순서가 잘못된게 아닐까? Replication이 왜 Sentinel 보다 뒤에....  
어쨌든 replication으로 HA와 failover를 만족해보자  

**replication 시스템의 매커니즘**  
1. master - replica 가 잘 연결되었다면, 마스터는 마스터 쪽에 발생하는 dataset happening(client writes, keys expired etc..)을 레플리카에 반영하기 위해서 command stream 전송하여 계속 update한다.
2. master - replica의 link가 break 되었다가 다시 connect 되면 break 동안 발생한 command들만 partial resynchronization하려고 시도한다.
3. 2가 불가능하다면 full resynchronization을 시도한다. master는 data snapshot을 찍어서 replica에게 전송하고 데이터 변경에 대한 command stream을 전송한다.

replication은 기본적으로 비동기로 수행된다. 그래서 master는 replica에 무언가 반영되는 것을 기다려 주지 않는다. 그런데 동기로 수행하기 위한 몇가지 옵션도 제공한다.  

### Important facts about Redis replication
* redis 는 비동기로 레플리카에 저장된 데이터를 확인한다.
* master : replica / 1 : N
* replica - replica 간 계층 구조를 가질 수 있다. 
* replica의 동기화 작업이 master에 들어오는 query를 block하지 않는다.
* replica도 non-blocking이다. replica가 synchronization 하고 있어도 기존 데이터에 쿼리하는 것은 수행할 수 잇다.
* replica를 두가지 역할로 사용할 수 있다. read-only query 를 수행하기 위해서 or HA
* master가 모든 데이터를 disk에 저장하지 않기 위해서 replication을 사용할 수 있다. 이 방법은 master가 재시작 되었을 때 데이터가 모두 날아간 상태에서 replica가 sync를 시도하니 조심해야한다.

### Safety of replication when master has persistence turned off
replication을 사용하면 persistence option을 켜는 것을 강력하게 권장한다.  
Sentinel 써도 데이터 유실 보장 어렵다. Sentinel이 master down을 감지하기 전에 재시작 되버릴 수 있으니깐.  
persistence 사용안할꺼면 master에 auto restart라고 꺼라.  

### How Redis replication works
Redis master는 random string으로 이루어진 replication ID를 가진다. 또, replica에게 데이터를 전송할 때 offset을 가진다.
`Replication ID, offset`
  
replica들은 연결되면 `PSYNC` 명령어를 보내서 old replication ID와 offset을 보낸다. 이 offset을 보고 partial sync가 가능하다.  
master에 backlog가 충분하지 않거나 replication ID가 unknown이면 full resync 드간다.  

**how full sync works**  
1. background에서 data saving process 진행한다. 
2. 그와 동시에 new wirte command를 모두 buffer한다.  
3. 1이 완료되면 데이터를 replica에게 보낸다.
4. 2도 보낸다.

telnet으로 직접 수행할 수도 있다. (내가 왜?) 

### Replication ID explained
same replication ID, offset이면 same data이다. 그러나 항상 같은 시간에 같은 데이터는 아니다.  
instance A offset 1000, instance B offset 1023 이면 같은 시간에 instance A에는 command가 적게 도착했다는 뜻

### Diskless replication
이거 쓰지마라.

### Configuration
replication을 사용하고싶다면 `replicaof {master ip} {port}`을 설정 파일에 추가해라.  

### Read-only  replica
`replica-read-only` 옵션을 enable해서 사용하자. writable replica 사용을 추천하지 않는다.

### Setting a replica to authenticate to a master
패스워드 설정도 할 수 있다.
redis cli:`config set masterauth <password>` or config file: `masterauth <password>`

### Allow writes only with N attached replicas
레플리카는 비동기로 동작하기 때문에 레플리카에 데이터가 잘 복제 되었는지 보장이 안된다. 그래서 항상 data loss를 위한 window가 있다.  
이게 왜 필요하지? 복제가 되든안되든 write하는 것과 무슨 상관? write하면 replica에 부하가 발생해서 그런가?

### hHow Redis replication deals with expires on keys
* replica는 직접 expire 하지 않고 master의 키가 expire 되는 것을 기다린다. master 가 expire되면 모든 replica에 DEL 명령어를 전송한다.
* 근데 이 방법을 사용하다보면 가끔 replica에 값이 남아 있는 경우가 있다. 그러면 레플리카는 논리적 시계를 사용해서 키가 존재하지 않는 것을 master에 보고한다. 
* 