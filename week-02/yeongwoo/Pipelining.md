# Pipelining

생성일: 2022년 12월 3일 오후 8:04

> 명령을 일괄 처리하여 왕복시간을 최적화하는 방법
> 
- 각 개별 명령에 대한 응답을 기다리지 않고 한 번에 명령을 내림으로써 성능을 향상시키는 기술이다

# ****Request/Response protocols and round-trip time (RTT)****

- Redis는 TCP 서버이다.
- 요청 완료 단계
    1. 클라이언트는 서버에 쿼리를 보내고 일반적으로 차단 방식으로 소켓에서 서버 응답을 읽는다.
    2. 서버는 명령을 처리하고 응답을 다시 클라이언트로 보낸다.
- 클라이언트와 서버는 네트워크 링크를 통해 연결된다.
    - 이 링크는 매우 빠르거나 매우 느릴 수 있음
- RTT: 왕복시간. 패킷이 클라이언트에서 서버로, 서버에서 다시 클라이언트로 응답 전달하는 시간.
    - 짧으면 짧을 수록 좋음

---

# Redis Pipelining

- pipelining: 클라이언트가 이전 응답을 읽지 않은 경우에 새 요청을 처리할 수 있도록 요청/응답 서버를 구현 가능
    - 응답을 전혀 기다리지 않고 서버에 여러 명령을 보낼 수 있고, 최정적으로 단일 단계에서 응답을 읽을 수 있다.
- 중요사항
    - 클라이언트가 파이프 라이닝을 사용하여 명령을 보내는 동안 서버는 메모리를 사용하여 응답을 대기열에 강제로 배치한다.
    - 따라서 파이프라이닝을 사용해 많은 명령을 보내야 하는 경우 적절한 수로 나누어 여러번 보내는 것이 좋다.
    - 속도는 거의 동일하지만 사용되는 추가 메모리는 10,000개의 명령ㅇ에 대한 응답을 대기시키는데 필요한 최대 양이다.

---

# ****It's not just a matter of RTT****

- 파이프라이닝은 왕복 시간과 관련된 대기 시간 비용을 줄이는 방법일 뿐만 아니라 실제로 Redis 서버에서 초당 수행할 수 있는 작업수를 크게 향상시킴
- 파이프 라이닝을 사용하지 않을 때
    - 데이터 구조에 액세스하고 응답을 생성하는 관점에서 매우 저렴
    - 읽기/쓰기 시스템 호출이 포함되며 사용자 영역에서 커널 영역으로 이동하는 socket I/O를 수행하는 관점에서 비용이 많이 든다.
    - 컨텍스트 스위칭은 엄청난 성능 저하임
- 파이프 라이닝을 사용할 때
    - 단일 읽기로 많은 명령을 읽고, 단일 쓰기로 여러 응답을 전달한다. 결과적으로 최대 10배정도 빨라진다.

---

# ****Pipelining vs Scripting****

- Redis scripting을 사용하면 서버측에서 필요한 많은 작업을 수행하는 스크립트를 사용해 파이프라이닝에 대한 여러 사용 사례를 보다 효율적으로 해결할 수 있다.
- Scripting
    - 장점
        - 최소한의 대기시간으로 데이터 읽기/쓰기가 가능해 읽기, 계산, 쓰기와 같은 작업을 매우 빠르게 수행할 수 있음.
- 파이프라이닝에서 EVAL 또는 EVALSHA 명령을 보내고싶을 때, SCRIPT LOAD 명령으로 지원 가능

---

# ****Appendix: Why are busy loops slow even on the loopback interface?****

> loopback 인터페이스에서 loop가 느린 이유
> 
- 서버와 클라이언트가 동일한 물리 기계에서 실행중일 때 루프백 인터페이스에서 느린 이유가 있다.

```bash
FOR-ONE-SECOND:
    Redis.SET("foo","bar")
END
```

- 시스템이 항상 실행되는 것은 아니기 때문. 프로세스를 실행하게 하는 것은 커널 스케쥴러이다.
- 벤치마킹을 할 때, loop benchmark는 하지 말자.