#!/bin/bash
# copilot-proxy-ctl.sh - Management script for copilot-proxy launchd service
# Usage: copilot-proxy-ctl.sh {start|stop|restart|status|logs|install|uninstall}

set -e

SERVICE_ID="pl.rrj.copilot-proxy"
PLIST_NAME="${SERVICE_ID}.plist"
PLIST_SRC="$(dirname "$0")/${PLIST_NAME}"
PLIST_DEST="${HOME}/Library/LaunchAgents/${PLIST_NAME}"
PID_FILE="${TMPDIR}copilot-proxy.pid"
LOG_FILE="${TMPDIR}copilot-proxy.log"
HEALTH_URL="http://127.0.0.1:11434/healthz"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

print_status() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}!${NC} $1"
}

check_installed() {
    if [ ! -f "$PLIST_DEST" ]; then
        print_error "Service not installed. Run '$0 install' first."
        exit 1
    fi
}

cmd_install() {
    if [ ! -f "$PLIST_SRC" ]; then
        print_error "Plist file not found: $PLIST_SRC"
        exit 1
    fi
    
    # Create LaunchAgents directory if it doesn't exist
    mkdir -p "${HOME}/Library/LaunchAgents"
    
    # Copy plist to LaunchAgents
    cp "$PLIST_SRC" "$PLIST_DEST"
    print_status "Installed plist to $PLIST_DEST"
    
    # Ensure the binary exists
    BINARY_PATH="$(dirname "$0")/../bin/copilot-proxy"
    if [ ! -f "$BINARY_PATH" ]; then
        print_warning "Binary not found at $BINARY_PATH"
        print_warning "Run 'make build' before starting the service"
    fi
}

cmd_uninstall() {
    # Stop service first if running
    if launchctl list | grep -q "$SERVICE_ID"; then
        cmd_stop
    fi
    
    if [ -f "$PLIST_DEST" ]; then
        rm "$PLIST_DEST"
        print_status "Removed plist from $PLIST_DEST"
    else
        print_warning "Plist not found at $PLIST_DEST"
    fi
}

cmd_start() {
    check_installed
    
    if launchctl list | grep -q "$SERVICE_ID"; then
        print_warning "Service is already running"
        return
    fi
    
    launchctl load "$PLIST_DEST"
    print_status "Service started"
    
    # Wait a moment and check health
    sleep 1
    cmd_health
}

cmd_stop() {
    check_installed
    
    if ! launchctl list | grep -q "$SERVICE_ID"; then
        print_warning "Service is not running"
        return
    fi
    
    launchctl unload "$PLIST_DEST"
    print_status "Service stopped"
}

cmd_restart() {
    check_installed
    
    if launchctl list | grep -q "$SERVICE_ID"; then
        cmd_stop
    fi
    sleep 1
    cmd_start
}

cmd_status() {
    check_installed
    
    echo "=== Service Status ==="
    if launchctl list | grep -q "$SERVICE_ID"; then
        print_status "Service is running"
        
        # Try to get PID
        PID=$(launchctl list | grep "$SERVICE_ID" | awk '{print $1}')
        if [ "$PID" != "-" ] && [ -n "$PID" ]; then
            echo "  PID: $PID"
        fi
    else
        print_error "Service is not running"
    fi
    
    echo ""
    cmd_health
    
    echo ""
    echo "=== Log File ==="
    if [ -f "$LOG_FILE" ]; then
        echo "  Location: $LOG_FILE"
        echo "  Size: $(du -h "$LOG_FILE" | cut -f1)"
    else
        print_warning "Log file not found"
    fi
}

cmd_health() {
    echo "=== Health Check ==="
    if curl -s --connect-timeout 2 "$HEALTH_URL" > /dev/null 2>&1; then
        RESPONSE=$(curl -s "$HEALTH_URL")
        print_status "Health endpoint responding"
        echo "  Response: $RESPONSE"
    else
        print_error "Health endpoint not responding at $HEALTH_URL"
    fi
}

cmd_logs() {
    if [ -f "$LOG_FILE" ]; then
        echo "=== Tailing $LOG_FILE (Ctrl+C to stop) ==="
        tail -f "$LOG_FILE"
    else
        print_error "Log file not found: $LOG_FILE"
        exit 1
    fi
}

cmd_help() {
    echo "copilot-proxy-ctl.sh - Management script for copilot-proxy"
    echo ""
    echo "Usage: $0 {command}"
    echo ""
    echo "Commands:"
    echo "  install    Install the launchd plist to ~/Library/LaunchAgents"
    echo "  uninstall  Remove the launchd plist"
    echo "  start      Start the service"
    echo "  stop       Stop the service"
    echo "  restart    Restart the service"
    echo "  status     Show service status and health"
    echo "  logs       Tail the log file"
    echo "  help       Show this help message"
}

# Main
case "${1:-}" in
    install)
        cmd_install
        ;;
    uninstall)
        cmd_uninstall
        ;;
    start)
        cmd_start
        ;;
    stop)
        cmd_stop
        ;;
    restart)
        cmd_restart
        ;;
    status)
        cmd_status
        ;;
    logs)
        cmd_logs
        ;;
    health)
        cmd_health
        ;;
    help|--help|-h)
        cmd_help
        ;;
    *)
        cmd_help
        exit 1
        ;;
esac
