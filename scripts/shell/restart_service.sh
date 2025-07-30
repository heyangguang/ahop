#!/bin/bash
# Shell script for restarting services
# This script can restart various services based on the service_name parameter
# Usage: ./restart_service.sh <service_name>

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Get service name from parameter or environment variable
SERVICE_NAME="${1:-${SERVICE_NAME}}"

# Validate service name
if [ -z "$SERVICE_NAME" ]; then
    print_error "Service name is required"
    echo "Usage: $0 <service_name>"
    echo "Or set SERVICE_NAME environment variable"
    exit 1
fi

print_info "Starting restart process for service: $SERVICE_NAME"

# Check if service exists
if ! systemctl list-unit-files | grep -q "^${SERVICE_NAME}.service"; then
    print_error "Service $SERVICE_NAME does not exist"
    exit 1
fi

# Get current status
print_info "Checking current status of $SERVICE_NAME"
CURRENT_STATUS=$(systemctl is-active "$SERVICE_NAME" 2>/dev/null || echo "inactive")
print_info "Current status: $CURRENT_STATUS"

# Stop the service
print_info "Stopping $SERVICE_NAME..."
if systemctl stop "$SERVICE_NAME"; then
    print_info "Service stopped successfully"
else
    print_error "Failed to stop service"
    exit 1
fi

# Wait a moment
sleep 3

# Start the service
print_info "Starting $SERVICE_NAME..."
if systemctl start "$SERVICE_NAME"; then
    print_info "Service started successfully"
else
    print_error "Failed to start service"
    exit 1
fi

# Enable the service to start on boot
print_info "Enabling $SERVICE_NAME to start on boot..."
systemctl enable "$SERVICE_NAME" >/dev/null 2>&1 || true

# Wait for service to be fully up
print_info "Waiting for service to be fully operational..."
MAX_ATTEMPTS=12  # 60 seconds total
ATTEMPT=0

while [ $ATTEMPT -lt $MAX_ATTEMPTS ]; do
    if systemctl is-active --quiet "$SERVICE_NAME"; then
        print_info "Service is active"
        break
    fi
    ATTEMPT=$((ATTEMPT + 1))
    sleep 5
done

# Final status check
FINAL_STATUS=$(systemctl is-active "$SERVICE_NAME" 2>/dev/null || echo "inactive")
if [ "$FINAL_STATUS" = "active" ]; then
    print_info "Service $SERVICE_NAME restarted successfully and is now active"
    
    # Perform service-specific health checks
    case "$SERVICE_NAME" in
        nginx)
            if nginx -t >/dev/null 2>&1; then
                print_info "Nginx configuration test passed"
            else
                print_warning "Nginx configuration test failed, but service is running"
            fi
            ;;
        mysql|mariadb)
            if mysqladmin ping >/dev/null 2>&1; then
                print_info "MySQL/MariaDB is responding to ping"
            else
                print_warning "MySQL/MariaDB ping failed, but service is running"
            fi
            ;;
        redis|redis-server)
            if redis-cli ping >/dev/null 2>&1; then
                print_info "Redis is responding to ping"
            else
                print_warning "Redis ping failed, but service is running"
            fi
            ;;
        postgresql)
            if sudo -u postgres pg_isready >/dev/null 2>&1; then
                print_info "PostgreSQL is ready"
            else
                print_warning "PostgreSQL readiness check failed, but service is running"
            fi
            ;;
    esac
    
    # Show service details
    print_info "Service details:"
    systemctl status "$SERVICE_NAME" --no-pager | head -n 10
    
    exit 0
else
    print_error "Service $SERVICE_NAME failed to restart properly"
    print_error "Final status: $FINAL_STATUS"
    
    # Show error logs
    print_error "Recent logs:"
    journalctl -u "$SERVICE_NAME" -n 20 --no-pager
    
    exit 1
fi