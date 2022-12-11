# Pub/Sub

생성일: 2022년 12월 11일 오후 5:03

- SUBSCRIBE, UNSUBSCRIBE, PUBLISH는 발신자(publisher)가 특정 수신자(subscribers)에게 메시지를 보내도록 프로그래밍되지 않은 Pub/Sub 메시징 패러다임을 구현함.
- published된 메시지들은 구독자가 누구인지 알지 못한 상태로 채널로 특정지어진다.
- subscriber는 하나 이상의 채널에 관심을 표명하고 메시지를 수신한다.
- 예
    - 채널 foo와 bar를 구독하기 위해 클라이언트는 채널 이름을 제공하는 SUBSCRIBE를 발행한다.
        
        ```bash
        SUBSCRIBE foo bar
        ```
        
- 다른 클라이언트가 채널로 보낸 메시지는 Redis에서 구독한 모든 클라이언트로 푸시됨.
- 하나 이상에 가입한 클라이언트는 다른 채널에 가입/탈퇴할 수 있지만 명령을 실행해서는 안된다.
- 구독/구독 취소작업에 대한 응답은 메시지 형식으로 전송되므로 클라이언트는 첫 번째 요소가 메시지 유형을 나타내는 일관된 메시지 스트림을 읽을 수 있다.
    - 허용되는 명령
        - SUBSCIRBE
        - SSUBSCRIBE
        - SUNSUBSCRIBE
        - PSUBSCRIBE
        - UNSUBSCRIBE
        - PUNSHCRIBE
        - PING
        - RESET
        - QUIT
- redis-cli는 subscribed mode에서 명령을 허용하지 않고 Ctrl+C로만 모드를 종료가능하다.

# Format of pushed message

- 메시지는 세가지 요소가 있는 배열 응답이다.
- 첫 번째 요소의 메시지의 종류
    - subscribe: 회신의 두번째 요소로 제공된 채널을 성공적으로 구독했음을 의미
        - 세번째 argument는 구독중인 채널 수 나타냄
    - unsubscribe: 회신에서 두 번째 요소로 제공된 채널에서 구독을 성공적으로 취소했음을 의미
        - 세번째 argument는 구독중인 채널 수 나타냄
        - 마지막 인수가 0이면 채널 구독 하지 않고, 클라이언트는 Pub/Sub 상태 밖에 있으므로 모든 종류의 명령 실행가능.
    - message: 다른 클라이언트에서 발행한 PUBLISH 명령 결과로 받은 메시지
        - 두번째 argument는 실제 메시지 페이로드이다.

# ****Database & Scoping****

- Pub/Sub은 키공간과 관련이 없다. 데이터베이스 번호를 포함해 모든 수준에서 간섭하지 않도록 만들어짐

# ****Pattern-matching subscriptions****

- 주어진 패턴과 일치하는 채널 이름으로 전송된 모든 메시지를 수신 가능하다.
- glob 스타일 패턴을 구독한다.
    
    ```bash
    # 패턴 구독
    PSUBSCRIBE news.*
    
    # 패턴 구독 취소
    PUNSUBSCRIBE news.*
    ```
    
- pattern-matching으로 받은 메시지는 다른형식으로 전송된다.
    - 메시지 유형은 pmessage이다.
    - pattern-matching된 다른 클라이언트가 발행한 PUBLISH 명령의 결과로 수신된 메시지이다.
    - 두 번째 요소는 일치된 원래 패턴
    - 세 번째 요소는 원래채널의 이름
    - 마지막 요소는 실제 메시지 페이로드

# ****Messages matching both a pattern and a channel subscription****

- 게시된 메시지와 일치하는 여러 패턴을 구독하거나 메시지와 일치하는 패턴 및 채널 모두를 구독하는 경우 클라이언트는 단일 메시지를 여러 번 수신할 수 있다.
    
    ```bash
    # 채널 foo로 메시지를 보내면 클라이언트는 두 개의 메시지를 받게 됩니다.
    # 하나는 message 유형이고 다른 하나는 pmmessage 유형입니다.
    SUBSCRIBE foo
    PSUBSCRIBE f*
    ```
    

# ****The meaning of the subscription count with pattern matching****

- subscribe, unsubscribe, psubscribe 및 punsubscribe 메시지 유형에서 마지막 인수는 아직 활성화된 구독 수이다. 이 숫자는 실제로 클라이언트가 아직 구독 중인 총 채널 및 패턴 수입니다.
- 클라이언트는 모든 채널 및 패턴에서 구독을 취소한 결과 이 수가 0으로 떨어질 때만 Pub/Sub 상태를 종료합니다.

# ****Sharded Pub/Sub****

- 7.0부터 슬롯에 키를 할당하는 데 사용되는 것과 동일한 알고리즘에 의해 샤드 채널이 슬롯에 할당되는 샤드된 Pub/Sub가 도입됨.
- 샤드 채널이 해시된 슬롯을 소유한 노드로 샤드 메시지를 보내야 합니다.
- 클러스터는 게시된 샤드 메시지가 샤드의 모든 노드로 전달되는지 확인하므로 클라이언트는 슬롯을 담당하는 마스터 또는 복제본 중 하나에 연결하여 샤드 채널을 구독할 수 있습니다. SSUBSCRIBE, SUNSUBSCRIBE 및 SPUBLISH는 분할된 Pub/Sub를 구현하는 데 사용됩니다.
- Sharded Pub/Sub 장점: 클러스터 모드에서 Pub/Sub 사용량을 확장하는 데 유리.
    - 클러스터의 샤드 내에서 메시지 전파를 제한합니다. 따라서 클러스터 버스를 통과하는 데이터의 양은 각 메시지가 클러스터의 각 노드로 전파되는 글로벌 Pub/Sub에 비해 제한됩니다. 이를 통해 사용자는 더 많은 샤드를 추가하여 Pub/Sub 사용량을 수평적으로 확장할 수 있습니다.