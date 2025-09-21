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
LOCAL_PORT=8080

PUBLIC_SERVICE=http://${PUBLIC_HOST}:${PUBLIC_PORT}
LOCAL_SERVICE=http://${LOCAL_HOST}:${LOCAL_PORT}

export WIREPORT_PROFILE=dev-meta

echo "🧪 Wireport HTTP Tunnel Test"
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

echo "📋 Step 3: Starting local HTTP server..."
# Create a simple HTTP server that responds with "OK"
python3 -c "
from http.server import HTTPServer, BaseHTTPRequestHandler

class H(BaseHTTPRequestHandler):
    def do_GET(self):
        self.send_response(200)
        self.end_headers()
        self.wfile.write(b'OK')
    def log_message(self, format, *args):
        if '${VERBOSE}' == 'true':
            print(f'[HTTP-SERVER] {format % args}')

HTTPServer(('${LOCAL_HOST}', ${LOCAL_PORT}), H).serve_forever()
" &

SERVER_PID=$!
echo "✅ Local HTTP server started (PID: $SERVER_PID)"
echo ""

echo "📋 Step 4: Waiting for service to be ready..."
if [[ "$VERBOSE" == "true" ]]; then
    echo "  Waiting 3 seconds for service to initialize..."
fi
sleep 3
echo ""

echo "📋 Step 5: Testing HTTP tunnel connectivity..."
SUCCESS_COUNT=0
TOTAL_TESTS=3

for i in {1..3}; do
    printf "  Test $i/3: "
    response=$(curl -s --connect-timeout 5 --max-time 10 "${PUBLIC_SERVICE}/" 2>/dev/null)
    
    if [[ "$response" == "OK" ]]; then
        echo "✅ PASS"
        ((SUCCESS_COUNT++))
        if [[ "$VERBOSE" == "true" ]]; then
            echo "    Response: $response"
        fi
    else
        echo "❌ FAIL"
        if [[ "$VERBOSE" == "true" ]]; then
            echo "    Response: $response"
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
    echo "🎉 ALL TESTS PASSED! HTTP tunnel is working correctly."
    exit 0
else
    echo "💥 SOME TESTS FAILED! HTTP tunnel has issues."
    exit 1
fi
