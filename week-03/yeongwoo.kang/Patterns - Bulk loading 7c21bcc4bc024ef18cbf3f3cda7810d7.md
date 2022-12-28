# Patterns - Bulk loading

생성일: 2022년 12월 18일 오전 11:21

> Redis 프로토콜을 사용해 대량으로 데이터 쓰기
> 
- 벌크로딩: 이미 존재하는 대량의 데이터를 Redis에 로딩하는 과정이다.

# Bulk loadingusing the Redis protocol

- 일반 Redis 클라이언트를 사용해 Bulk loading하는 것은 좋은 생각이 아님
    - 모든 명령에 대해 왕복시간을 소요해야하기 때문에 느리다.
    - 파이프라이닝을 사용할 수 있지만 가능한 할 빨리 삽입하는지 확인하기 위해 동시에 응답을 읽는 동안 새 명령을 작성해야한다.
- 소수의 클라이언트가 non-blocking I/O를 지원하며 모든 클라이언트가 처리량을 최대화하기 위해 효율적인 방식으로 응답을 구문 분석할 수 있는 것은 아니다.
- Bulk loading의 기본 방법: 데이터를 삽입하는 데 필요한 명령을 호출하기 위해 원시 형식의 Redis 프로토콜이 포함된 텍스트 파일을 생성하는 것이다.
    - 예: keyN → ValueN 형식의 수십억 개의 키가 있는 대규모 데이터 세트를 생성해야하는 경우 Redis 프로토콜 형식으로 다음 명령을 포함하는 파일을 생성한다.
        
        ```bash
        SET Key0 Value0
        SET Key1 Value1
        ...
        SET KeyN ValueN
        ```
        
        - 위 파일이 생성되면 Redis에 파일을 제공한다.
        
        ```bash
        # 구식방법. 모든 데이터가 언제 전송되었는지, 오류를 확인할 수 없음
        (cat data.txt; sleep 10) | nc localhost 6379 > /dev/null
        # 2.6버전 이상에서 파이프모드를 활용한 최신버전.
        cat data.txt | redis-cli --pipe
        ```
        
        - 다음과 같은 결과가 출력된다
        
        ```bash
        All data transferred. Waiting for the last reply...
        Last reply received from server.
        errors: 0, replies: 1000000
        ```
        
        - 만약 오류가 있다면, Redis instance에서 받은 오류만 표준 출력으로 리디렉션한다.

## Generating Redis Protocol

- Redis 프로토콜은 생성 및 구문 분석이 매우 간단함 ([관련 문서](https://redis.io/docs/reference/protocol-spec/))
- bulk-loading을 외한 프로토콜을 생성하는 명령은 다음과 같은 방식으로 표현됨
    
    ```bash
    *<args><cr><lf>
    $<len><cr><lf>
    <arg0><cr><lf>
    <arg1><cr><lf>
    ...
    <argN><cr><lf>
    ```
    
    - 여기서 `<cr>`은 “\r”을 의미하고 `<lf>`은 "\n"을 의미한다.
    - 예: SET 명령어 키 값은 다음 프로토콜로 표시됨
        
        ```bash
        *3<cr><lf>
        $3<cr><lf>
        SET<cr><lf>
        $3<cr><lf>
        key<cr><lf>
        $5<cr><lf>
        value<cr><lf>
        
        # 또는 인용된 문자열
        "*3\r\n$3\r\nSET\r\n$3\r\nkey\r\n$5\r\nvalue\r\n"
        ```
        
- 대량 로드를 위해 생성해야 하는 일은 위의 방식으로 표시된 명령으로 하나씩 구성되어 있다.
    - 예: Ruby함수
        
        ```bash
        def gen_redis_proto(*cmd)
            proto = ""
            proto << "*"+cmd.length.to_s+"\r\n"
            cmd.each{|arg|
                proto << "$"+arg.to_s.bytesize.to_s+"\r\n"
                proto << arg.to_s+"\r\n"
            }
            proto
        end
        
        puts gen_redis_proto("SET","mykey","Hello World!").inspect
        ```
        
        - 사용 법
        
        ```bash
        (0...1000).each{|n|
            STDOUT.write(gen_redis_proto("SET","Key#{n}","Value#{n}"))
        }
        ```
        
        - redis-cli를 사용해 대량 데이터를 가져오는 명령
        
        ```bash
        $ ruby proto.rb | redis-cli --pipe
        All data transferred. Waiting for the last reply...
        Last reply received from server.
        errors: 0, replies: 1000
        ```
        

## How the pipe mode works under the hood

- 파이프 모드는 netcat만큼 빠르고, 동시에 서버에서 마지막 응답을 보낸 시간을 이해할 수 있어야 한다.
- 방법
    - `redis-cli pipe`명령어는 서버에 최대한 빠르게 데이터를 보내려고 한다.
    - 동시에 사용 가능한 데이터를 읽고 구문 분석을 시도한다.
    - stdin에서 더 이상 읽을 데이터가 없으면 임의의 20바이트 문자열이 포함된 특수 `ECHO`명령을 보낸다.
    - 이 최종 명령이 전송되면 응답을 받는 코드는 20바이트와 일치하는 응답을 시작한다. 일치하는 응답에 도달하면 성공적으로 종료할 수 있다.
- 위 방법을 이용하면 “우리가 보내는 명령의 수”를 이해하기 위해 서버에 보내는 프로토콜을 구문 분석할 필요가 없고 응답만 볼 수 있다.
- 회신을 구문 분석하는 동안 우리는 구문 분석된 모든 회신의 카운터를 취하여 결국 대량 삽입 세션에 의해 서버로 전송된 명령의 양을 사용자에게 알릴 수 있다.