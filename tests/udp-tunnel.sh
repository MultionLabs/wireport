#!/bin/bash

# Parse command line arguments
VERBOSE=false
PUBLIC_HOST=""

while [[ $# -gt 0 ]]; do
    case $1 in
        -v|--verbose)
            VERBOSE=true
            shift
            ;;
        *)
            if [[ -z "$PUBLIC_HOST" ]]; then
                PUBLIC_HOST=$1
            fi
            shift
            ;;
    esac
done

if [[ -z "$PUBLIC_HOST" ]]; then
    echo "Usage: $0 [-v|--verbose] <public_host>"
    echo "  -v, --verbose    Show detailed output and logs"
    exit 1
fi

PUBLIC_PORT=32421
LOCAL_HOST=10.0.0.2
LOCAL_PORT=9999

PUBLIC_SERVICE=udp://${PUBLIC_HOST}:${PUBLIC_PORT}
LOCAL_SERVICE=udp://${LOCAL_HOST}:${LOCAL_PORT}

export WIREPORT_PROFILE=dev-meta

echo "🧪 Wireport UDP Tunnel Test"
echo "================================"
echo "Public:  ${PUBLIC_SERVICE}"
echo "Local:   ${LOCAL_SERVICE}"
if [[ "$VERBOSE" == "true" ]]; then
    echo "Mode:    VERBOSE"
fi
echo ""

echo "📋 Step 1: Checking wireport version..."
if [[ "$VERBOSE" == "true" ]]; then
    wireport --version
else
    wireport --version | head -1
fi
echo ""

echo "📋 Step 2: Publishing service..."
if [[ "$VERBOSE" == "true" ]]; then
    wireport service publish --public ${PUBLIC_SERVICE} --local ${LOCAL_SERVICE}
    PUBLISH_EXIT_CODE=$?
else
    wireport service publish --public ${PUBLIC_SERVICE} --local ${LOCAL_SERVICE} >/dev/null 2>&1
    PUBLISH_EXIT_CODE=$?
fi

if [ $PUBLISH_EXIT_CODE -eq 0 ]; then
    echo "✅ Service published successfully"
else
    echo "❌ Failed to publish service"
    exit 1
fi
echo ""

echo "📋 Step 3: Starting local UDP echo server..."
# Create a simple UDP echo server that responds with "OK" to any message
python3 -c "
import socket
import sys

def udp_echo_server():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(('${LOCAL_HOST}', ${LOCAL_PORT}))
    if '${VERBOSE}' == 'true':
        print(f'[UDP-SERVER] Listening on ${LOCAL_HOST}:${LOCAL_PORT}')
    
    try:
        while True:
            data, addr = sock.recvfrom(1024)
            if '${VERBOSE}' == 'true':
                print(f'[UDP-SERVER] Received from {addr}: {data.decode()}')
            # Echo back 'OK' response
            sock.sendto(b'OK', addr)
            if '${VERBOSE}' == 'true':
                print(f'[UDP-SERVER] Sent to {addr}: OK')
    except KeyboardInterrupt:
        pass
    finally:
        sock.close()

udp_echo_server()
" &

SERVER_PID=$!
echo "✅ Local UDP server started (PID: $SERVER_PID)"
echo ""

echo "📋 Step 4: Waiting for service to be ready..."
if [[ "$VERBOSE" == "true" ]]; then
    echo "  Waiting 3 seconds for service to initialize..."
fi
sleep 3
echo ""

echo "📋 Step 5: Testing UDP tunnel connectivity..."
SUCCESS_COUNT=0
TOTAL_TESTS=3

for i in {1..3}; do
    printf "  Test $i/3: "
    
    # Send UDP message and wait for response
    test_message="TEST_MESSAGE_$i"
    response=$(echo "$test_message" | nc -u -w 5 ${PUBLIC_HOST} ${PUBLIC_PORT} 2>/dev/null)
    
    if [[ "$response" == "OK" ]]; then
        echo "✅ PASS"
        ((SUCCESS_COUNT++))
        if [[ "$VERBOSE" == "true" ]]; then
            echo "    Sent: $test_message"
            echo "    Received: $response"
        fi
    else
        echo "❌ FAIL"
        if [[ "$VERBOSE" == "true" ]]; then
            echo "    Sent: $test_message"
            echo "    Received: $response"
        fi
    fi
    sleep 1
done
echo ""

echo "📋 Step 6: Cleaning up..."
if [[ "$VERBOSE" == "true" ]]; then
    echo "  Unpublishing service..."
    wireport service unpublish --public ${PUBLIC_SERVICE}
    echo "  Stopping local server (PID: $SERVER_PID)..."
    kill -9 $SERVER_PID
else
    wireport service unpublish --public ${PUBLIC_SERVICE} >/dev/null 2>&1
    kill -9 $SERVER_PID >/dev/null 2>&1
fi
echo "✅ Cleanup completed"
echo ""

echo "📊 Test Results"
echo "==============="
echo "Tests passed: $SUCCESS_COUNT/$TOTAL_TESTS"

if [ $SUCCESS_COUNT -eq $TOTAL_TESTS ]; then
    echo "🎉 ALL TESTS PASSED! UDP tunnel is working correctly."
    exit 0
else
    echo "💥 SOME TESTS FAILED! UDP tunnel has issues."
    exit 1
fi
