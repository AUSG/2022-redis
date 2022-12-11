# Transaction

생성일: 2022년 12월 11일 오후 6:46

- Redis 트랜잭션을 사용하면 한 단계에서 명령 그룹을 사용할 수 있다.
- command: MULTI, EXEC, DISCARD, WATCH
- 보장
    - 직렬화되어 순차적으로 실행된다. (단일 격리 작업)
    - EXEC명령은 트랜잭션의 모든 명령 실행을 트리거하므로 클라이언트가 EXEC 명령을 호출하기 전에 트랜잭션 컨텍스트에서 서버에 대한 연결이 끊어지면 아무 작업도 수행되지 않는다.
- 낙관적 잠금형태로 보증을 허용한다.

# Usage

- 트랜잭션은 MULTI 명령을 사용하여 입력된다.
- 명령은 항상 OK로 응답한다.
- 사용자는 여러 명령을 실행할 수 있다. 이 명령들은 Redis내 대기열에 넣고, EXEC가 호출되면 모든 명령이 실행된다.
- 대신 DISCARD를 호출하면 트랜잭션 대기열이 비워지고 트랙잭션이 종료된다.
- 예: foo와 bar의 값을 1씩 증가
    
    ```bash
    > MULTI
    OK
    > INCR foo
    QUEUED
    > INCR bar
    QUEUED
    > EXEC
    1) (integer) 1
    2) (integer) 1
    ```
    
- EXEC는 각 명령어의 결과가 담긴 응답 배열을 반환한다.

# ****Errors inside a transaction****

- 트랜잭션 중에 두 가지 종류의 명령 오류가 발생할 수 있다.
    - 명령이 대기열에 저장되지 않을 수 있으므로 EXEC 명령어가 호출되기 전에 오류가 발생할 수 있다.
        - 잘못된 구문, 메모리 부족 등
    - 잘못된 값을 가진 키에 대해 작업을 수행하면 EXEC가 호출된 후 명령이 실패할 수 있다.
- 명령오류가 발생하면 트랜잭션 실행을 abort하고 트랜잭션을 버린다.
- EXEC 이후에 발생하는 오류는 특별한 방식으로 처리되지 않는다. 다른 모든 명령은 트랜잭션 중에 일부 명령이 실패하더라도 실행된다.
    
    ```bash
    Trying 127.0.0.1...
    Connected to localhost.
    Escape character is '^]'.
    MULTI
    +OK
    SET a abc
    +QUEUED
    LPOP a
    +QUEUED
    EXEC
    *2
    +OK
    -ERR Operation against a key holding the wrong kind of value
    ```
    
- EXEC는 하나의 OK코드이고, 다른 하나는 -ERR응답인 두 요소의 대량 문자열 응답을 반환했다.
- 롤백을 지원하지 않는다.

# ****Discarding the command queue****

- DISCARD: 트랜잭션 중단 명령어. 명령이 실행되지 않고 연결상태가 정상으로 복원됨
    
    ```bash
    > SET foo 1
    OK
    > MULTI
    OK
    > INCR foo
    QUEUED
    > DISCARD
    OK
    > GET foo
    "1"
    ```
    

# ****Optimistic locking using check-and-set****

- WATCH: 트랜잭션 CAS(확인 및 설정)동작을 제공하는데 사용되는 명령어
- WATCHed 키는 이에대한 변경사항을 감지하기 위해 모니터링됨. EXEC 명령어 전에 하나 이상의 감시 키가 수정되면 전체 트랜잭션이 중단되고 EXEC는 Null 응답을 반환하여 트랜잭션이 실패했음을 알린다.
- 예: 키값을 원자적으로 1씩 증가시켜야한다.
    
    ```bash
    val = GET mykey
    val = val + 1
    SET mykey $val
    ```
    
    - 단일 클라이언트가 아닌 여러 클라이언트가 동시에 키를 증가시키려고 하면 경쟁조건이 발생한다.
    
    ```bash
    WATCH mykey
    val = GET mykey
    val = val + 1
    MULTI
    SET mykey $val
    EXEC
    ```
    
    - WATCH호출과 EXEC호출 사이의 시간에 val의 결과를 수정하면 트랜잭션이 실패한다.

# WATCH explained

- EXEC를 조건부로 만드는 명령이다.
- 여러번 호출 가능하다.

# Redis scripting and transactions

- 트랜잭션으로 수행할 수 있는 모든 작업은 스크립트로 작업 가능하다.
- 일반적으로 스크립트는 더 간단하고 빠르다.