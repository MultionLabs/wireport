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

PUBLIC_PORT=32420
LOCAL_HOST=10.0.0.2
LOCAL_PORT=8000

PUBLIC_SERVICE=tcp://${PUBLIC_HOST}:${PUBLIC_PORT}
LOCAL_SERVICE=tcp://${LOCAL_HOST}:${LOCAL_PORT}

export WIREPORT_PROFILE=dev-meta

echo "🧪 Wireport TCP Tunnel Test"
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

echo "📋 Step 3: Starting local TCP server..."
if [[ "$VERBOSE" == "true" ]]; then
    python3 -c "
import socket
import sys

def tcp_server():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(('${LOCAL_HOST}', ${LOCAL_PORT}))
    sock.listen(1)
    print(f'[TCP-SERVER] Listening on ${LOCAL_HOST}:${LOCAL_PORT}')
    
    try:
        while True:
            conn, addr = sock.accept()
            print(f'[TCP-SERVER] Connection from {addr}')
            data = conn.recv(1024)
            print(f'[TCP-SERVER] Received: {data.decode()}')
            conn.send(b'OK')
            print(f'[TCP-SERVER] Sent: OK')
            conn.close()
    except KeyboardInterrupt:
        pass
    finally:
        sock.close()

tcp_server()
" &
else
    python3 -c "
import socket
import sys

def tcp_server():
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
    sock.bind(('${LOCAL_HOST}', ${LOCAL_PORT}))
    sock.listen(1)
    
    try:
        while True:
            conn, addr = sock.accept()
            data = conn.recv(1024)
            conn.send(b'OK')
            conn.close()
    except KeyboardInterrupt:
        pass
    finally:
        sock.close()

tcp_server()
" >/dev/null 2>&1 &
fi

SERVER_PID=$!
echo "✅ Local TCP server started (PID: $SERVER_PID)"
echo ""

echo "📋 Step 4: Waiting for service to be ready..."
if [[ "$VERBOSE" == "true" ]]; then
    echo "  Waiting 3 seconds for service to initialize..."
fi
sleep 3
echo ""

echo "📋 Step 5: Testing TCP tunnel connectivity..."
SUCCESS_COUNT=0
TOTAL_TESTS=3

for i in {1..3}; do
    printf "  Test $i/3: "
    
    # Use Python TCP client - much more reliable
    response=$(python3 -c "
import socket
import sys

try:
    sock = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
    sock.settimeout(5)
    sock.connect(('${PUBLIC_HOST}', ${PUBLIC_PORT}))
    sock.send(b'TEST_MESSAGE_$i')
    response = sock.recv(1024)
    sock.close()
    print(response.decode(), end='')
except:
    pass
" 2>/dev/null)
    
    if [[ "$response" == "OK" ]]; then
        echo "✅ PASS"
        ((SUCCESS_COUNT++))
        if [[ "$VERBOSE" == "true" ]]; then
            echo "    Sent: TEST_MESSAGE_$i"
            echo "    Received: $response"
        fi
    else
        echo "❌ FAIL"
        if [[ "$VERBOSE" == "true" ]]; then
            echo "    Sent: TEST_MESSAGE_$i"
            echo "    Received: '$response'"
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
    echo "🎉 ALL TESTS PASSED! TCP tunnel is working correctly."
    exit 0
else
    echo "💥 SOME TESTS FAILED! TCP tunnel has issues."
    exit 1
fi
