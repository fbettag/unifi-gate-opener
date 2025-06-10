#!/bin/bash

# UniFi Gate Opener Installation Script
# Supports: Linux (systemd/init.d), FreeBSD, OpenBSD, NetBSD
# Auto-detects OS, architecture, and service management system

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
GITHUB_REPO="fbettag/unifi-gate-opener"
APP_NAME="unifi-gate-opener"
APP_USER="unifi-gate-opener"
APP_GROUP="unifi-gate-opener"

# System paths
INSTALL_DIR="/opt/unifi-gate-opener"
CONFIG_DIR="/etc/unifi-gate-opener"
DATA_DIR="/var/lib/unifi-gate-opener"
LOG_DIR="/var/log/unifi-gate-opener"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
DATABASE_FILE="${DATA_DIR}/gate_opener.db"

# Default configuration
DEFAULT_PORT=8080

# Print functions
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}"
    echo "=================================================="
    echo "         UniFi Gate Opener Installer"
    echo "    Transform your UniFi network into an"
    echo "      intelligent gate controller"
    echo "=================================================="
    echo -e "${NC}"
}

# Detect OS and architecture
detect_system() {
    print_info "Detecting system information..."
    
    # Detect OS
    case "$(uname -s)" in
        Linux*)
            OS="linux"
            OS_PRETTY="Linux"
            ;;
        *)
            print_error "Unsupported operating system: $(uname -s)"
            print_error "This installer only supports Linux. SQLite with CGO is required."
            exit 1
            ;;
    esac
    
    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        aarch64|arm64)
            ARCH="arm64"
            ;;
        armv7l|armhf)
            ARCH="arm"
            ;;
        *)
            print_error "Unsupported architecture: $(uname -m)"
            print_error "Supported: amd64, arm64, arm (armv7)"
            exit 1
            ;;
    esac
    
    # Detect service management system
    if command -v systemctl >/dev/null 2>&1 && [ -d /etc/systemd/system ]; then
        SERVICE_MGR="systemd"
    elif [ -d /etc/init.d ]; then
        SERVICE_MGR="initd"
    else
        print_error "No supported service management system found"
        exit 1
    fi
    
    print_success "Detected: $OS_PRETTY $ARCH with $SERVICE_MGR"
}

# Check if running as root
check_root() {
    if [ "$EUID" -ne 0 ]; then
        print_error "This script must be run as root (use sudo)"
        exit 1
    fi
}

# Get latest release info
get_latest_release() {
    print_info "Fetching latest release information..."
    
    if command -v curl >/dev/null 2>&1; then
        RELEASE_INFO=$(curl -s "https://api.github.com/repos/$GITHUB_REPO/releases/latest")
    elif command -v wget >/dev/null 2>&1; then
        RELEASE_INFO=$(wget -qO- "https://api.github.com/repos/$GITHUB_REPO/releases/latest")
    else
        print_error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    # Extract version and download URL
    VERSION=$(echo "$RELEASE_INFO" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    DOWNLOAD_URL="https://github.com/$GITHUB_REPO/releases/download/$VERSION/unifi-gate-opener-$VERSION-$OS-$ARCH.tar.gz"
    
    if [ -z "$VERSION" ]; then
        print_error "Failed to get latest release information"
        exit 1
    fi
    
    print_success "Found latest version: $VERSION"
}

# Download and extract binary
download_binary() {
    print_info "Downloading UniFi Gate Opener $VERSION..."
    
    TMP_DIR=$(mktemp -d)
    cd "$TMP_DIR"
    
    if command -v curl >/dev/null 2>&1; then
        curl -L -o "unifi-gate-opener.tar.gz" "$DOWNLOAD_URL"
    else
        wget -O "unifi-gate-opener.tar.gz" "$DOWNLOAD_URL"
    fi
    
    print_info "Extracting binary..."
    tar -xzf "unifi-gate-opener.tar.gz"
    
    # Move binary to install directory
    mkdir -p "$INSTALL_DIR"
    cp "unifi-gate-opener-$OS-$ARCH" "$INSTALL_DIR/unifi-gate-opener"
    chmod +x "$INSTALL_DIR/unifi-gate-opener"
    
    # Cleanup
    cd /
    rm -rf "$TMP_DIR"
    
    print_success "Binary installed to $INSTALL_DIR/unifi-gate-opener"
}

# Create system user and group
create_user() {
    print_info "Creating system user and group..."
    
    case "$OS" in
        linux)
            if ! getent group "$APP_GROUP" >/dev/null 2>&1; then
                groupadd --system "$APP_GROUP"
            fi
            if ! getent passwd "$APP_USER" >/dev/null 2>&1; then
                useradd --system --gid "$APP_GROUP" --home-dir "$DATA_DIR" \
                        --shell /usr/sbin/nologin --comment "UniFi Gate Opener" "$APP_USER"
            fi
            ;;
        freebsd)
            if ! pw group show "$APP_GROUP" >/dev/null 2>&1; then
                pw group add "$APP_GROUP"
            fi
            if ! pw user show "$APP_USER" >/dev/null 2>&1; then
                pw user add "$APP_USER" -g "$APP_GROUP" -d "$DATA_DIR" \
                   -s /usr/sbin/nologin -c "UniFi Gate Opener"
            fi
            ;;
        openbsd|netbsd)
            if ! getent group "$APP_GROUP" >/dev/null 2>&1; then
                groupadd "$APP_GROUP"
            fi
            if ! getent passwd "$APP_USER" >/dev/null 2>&1; then
                useradd -g "$APP_GROUP" -d "$DATA_DIR" -s /sbin/nologin \
                        -c "UniFi Gate Opener" "$APP_USER"
            fi
            ;;
    esac
    
    print_success "Created user: $APP_USER"
}

# Create directories with proper permissions
create_directories() {
    print_info "Creating application directories..."
    
    mkdir -p "$CONFIG_DIR" "$DATA_DIR" "$LOG_DIR"
    
    chown root:root "$CONFIG_DIR"
    chmod 755 "$CONFIG_DIR"
    
    chown "$APP_USER:$APP_GROUP" "$DATA_DIR" "$LOG_DIR"
    chmod 750 "$DATA_DIR" "$LOG_DIR"
    
    print_success "Created directories with proper permissions"
}

# Interactive configuration
configure_app() {
    print_info "Configuring UniFi Gate Opener..."
    
    # Ask for port
    echo -n "Enter port number (default: $DEFAULT_PORT): "
    read -r PORT
    PORT=${PORT:-$DEFAULT_PORT}
    
    # Validate port
    if ! [[ "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1 ] || [ "$PORT" -gt 65535 ]; then
        print_warning "Invalid port number. Using default: $DEFAULT_PORT"
        PORT=$DEFAULT_PORT
    fi
    
    # Create basic configuration file
    cat > "$CONFIG_FILE" << EOF
# UniFi Gate Opener Configuration
# This file will be populated during the setup wizard
# Access the web interface at http://localhost:$PORT

database_path: "$DATABASE_FILE"
session_secret: "$(openssl rand -base64 32 2>/dev/null || dd if=/dev/urandom bs=32 count=1 2>/dev/null | base64 | tr -d '\n')"
setup_complete: false

# Default values - will be configured through web interface
admin:
  username: ""
  password_hash: ""

unifi:
  controller_url: ""
  username: ""
  password: ""
  site_id: "default"
  gate_ap_mac: ""
  poll_interval: 1

shelly:
  trigger_url: ""

gate:
  open_duration: 10
  log_activity: false

devices: []
EOF
    
    chown root:"$APP_GROUP" "$CONFIG_FILE"
    chmod 640 "$CONFIG_FILE"
    
    print_success "Configuration file created: $CONFIG_FILE"
    print_info "Port: $PORT"
}

# Create systemd service file
create_systemd_service() {
    print_info "Creating systemd service..."
    
    cat > /etc/systemd/system/unifi-gate-opener.service << EOF
[Unit]
Description=UniFi Gate Opener
Documentation=https://github.com/$GITHUB_REPO
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=$APP_USER
Group=$APP_GROUP
ExecStart=$INSTALL_DIR/unifi-gate-opener \\
    --config="$CONFIG_FILE" \\
    --database="$DATABASE_FILE" \\
    --port=$PORT \\
    --log-level=info \\
    --daemon
Restart=always
RestartSec=5
StartLimitIntervalSec=0

# Security settings
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=$DATA_DIR $LOG_DIR
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=unifi-gate-opener

[Install]
WantedBy=multi-user.target
EOF
    
    systemctl daemon-reload
    systemctl enable unifi-gate-opener
    
    print_success "Systemd service created and enabled"
}

# Create init.d service file
create_initd_service() {
    print_info "Creating init.d service..."
    
    cat > /etc/init.d/unifi-gate-opener << 'EOF'
#!/bin/sh
### BEGIN INIT INFO
# Provides:          unifi-gate-opener
# Required-Start:    $remote_fs $syslog $network
# Required-Stop:     $remote_fs $syslog $network
# Default-Start:     2 3 4 5
# Default-Stop:      0 1 6
# Short-Description: UniFi Gate Opener
# Description:       Intelligent gate controller using UniFi network
### END INIT INFO

. /lib/lsb/init-functions

USER="APP_USER_PLACEHOLDER"
DAEMON="INSTALL_DIR_PLACEHOLDER/unifi-gate-opener"
PIDFILE="/var/run/unifi-gate-opener.pid"
CONFIGFILE="CONFIG_FILE_PLACEHOLDER"
DATABASE="DATABASE_FILE_PLACEHOLDER"
PORT="PORT_PLACEHOLDER"

start() {
    log_daemon_msg "Starting UniFi Gate Opener" "unifi-gate-opener"
    start-stop-daemon --start --quiet --pidfile $PIDFILE --make-pidfile \
        --background --chuid $USER --exec $DAEMON -- \
        --config="$CONFIGFILE" --database="$DATABASE" --port=$PORT --daemon
    log_end_msg $?
}

stop() {
    log_daemon_msg "Stopping UniFi Gate Opener" "unifi-gate-opener"
    start-stop-daemon --stop --quiet --pidfile $PIDFILE
    rm -f $PIDFILE
    log_end_msg $?
}

case "$1" in
    start)
        start
        ;;
    stop)
        stop
        ;;
    restart)
        stop
        start
        ;;
    status)
        status_of_proc -p $PIDFILE "$DAEMON" "unifi-gate-opener"
        ;;
    *)
        echo "Usage: $0 {start|stop|restart|status}"
        exit 1
        ;;
esac

exit 0
EOF
    
    # Replace placeholders
    sed -i "s|APP_USER_PLACEHOLDER|$APP_USER|g" /etc/init.d/unifi-gate-opener
    sed -i "s|INSTALL_DIR_PLACEHOLDER|$INSTALL_DIR|g" /etc/init.d/unifi-gate-opener
    sed -i "s|CONFIG_FILE_PLACEHOLDER|$CONFIG_FILE|g" /etc/init.d/unifi-gate-opener
    sed -i "s|DATABASE_FILE_PLACEHOLDER|$DATABASE_FILE|g" /etc/init.d/unifi-gate-opener
    sed -i "s|PORT_PLACEHOLDER|$PORT|g" /etc/init.d/unifi-gate-opener
    
    chmod +x /etc/init.d/unifi-gate-opener
    
    # Enable service for different distributions
    if command -v update-rc.d >/dev/null 2>&1; then
        update-rc.d unifi-gate-opener defaults
    elif command -v chkconfig >/dev/null 2>&1; then
        chkconfig --add unifi-gate-opener
        chkconfig unifi-gate-opener on
    fi
    
    print_success "Init.d service created and enabled"
}

# Create rc.d service file (BSD systems)
create_rcd_service() {
    print_info "Creating rc.d service..."
    
    case "$OS" in
        freebsd)
            RC_DIR="/usr/local/etc/rc.d"
            ;;
        openbsd|netbsd)
            RC_DIR="/etc/rc.d"
            ;;
    esac
    
    cat > "$RC_DIR/unifi_gate_opener" << EOF
#!/bin/sh

# PROVIDE: unifi_gate_opener
# REQUIRE: LOGIN
# KEYWORD: shutdown

. /etc/rc.subr

name="unifi_gate_opener"
rcvar="unifi_gate_opener_enable"

command="$INSTALL_DIR/unifi-gate-opener"
command_args="--config=$CONFIG_FILE --database=$DATABASE_FILE --port=$PORT --daemon"
command_user="$APP_USER"
pidfile="/var/run/unifi-gate-opener.pid"

start_precmd="unifi_gate_opener_prestart"

unifi_gate_opener_prestart()
{
    touch \$pidfile
    chown $APP_USER \$pidfile
}

load_rc_config \$name
run_rc_command "\$1"
EOF
    
    chmod +x "$RC_DIR/unifi_gate_opener"
    
    # Add to rc.conf
    if [ "$OS" = "freebsd" ]; then
        sysrc unifi_gate_opener_enable="YES"
    else
        echo 'unifi_gate_opener_enable="YES"' >> /etc/rc.conf.local
    fi
    
    print_success "RC.d service created and enabled"
}

# Install service based on service manager
install_service() {
    case "$SERVICE_MGR" in
        systemd)
            create_systemd_service
            ;;
        initd)
            create_initd_service
            ;;
        rcd)
            create_rcd_service
            ;;
    esac
}

# Start service
start_service() {
    print_info "Starting UniFi Gate Opener service..."
    
    case "$SERVICE_MGR" in
        systemd)
            systemctl start unifi-gate-opener
            ;;
        initd)
            service unifi-gate-opener start
            ;;
        rcd)
            case "$OS" in
                freebsd)
                    service unifi_gate_opener start
                    ;;
                openbsd|netbsd)
                    /etc/rc.d/unifi_gate_opener start
                    ;;
            esac
            ;;
    esac
    
    print_success "Service started successfully"
}

# Check service status
check_service() {
    sleep 2
    print_info "Checking service status..."
    
    case "$SERVICE_MGR" in
        systemd)
            if systemctl is-active --quiet unifi-gate-opener; then
                print_success "Service is running"
            else
                print_warning "Service is not running. Check logs with: journalctl -u unifi-gate-opener"
            fi
            ;;
        initd)
            if service unifi-gate-opener status >/dev/null 2>&1; then
                print_success "Service is running"
            else
                print_warning "Service is not running. Check logs in $LOG_DIR"
            fi
            ;;
        rcd)
            print_info "Service installed. Check status with appropriate rc.d commands"
            ;;
    esac
}

# Print completion message
print_completion() {
    echo
    print_success "🎉 UniFi Gate Opener installation completed!"
    echo
    print_info "What's next:"
    echo "  1. Open your browser and go to: http://localhost:$PORT"
    echo "  2. Complete the setup wizard:"
    echo "     - Create admin account"
    echo "     - Configure UniFi controller connection"
    echo "     - Select gate access point"
    echo "     - Add authorized devices"
    echo
    print_info "Service management:"
    case "$SERVICE_MGR" in
        systemd)
            echo "  Start:   sudo systemctl start unifi-gate-opener"
            echo "  Stop:    sudo systemctl stop unifi-gate-opener"
            echo "  Restart: sudo systemctl restart unifi-gate-opener"
            echo "  Status:  sudo systemctl status unifi-gate-opener"
            echo "  Logs:    sudo journalctl -u unifi-gate-opener"
            ;;
        initd)
            echo "  Start:   sudo service unifi-gate-opener start"
            echo "  Stop:    sudo service unifi-gate-opener stop"
            echo "  Restart: sudo service unifi-gate-opener restart"
            echo "  Status:  sudo service unifi-gate-opener status"
            ;;
        rcd)
            case "$OS" in
                freebsd)
                    echo "  Start:   sudo service unifi_gate_opener start"
                    echo "  Stop:    sudo service unifi_gate_opener stop"
                    echo "  Restart: sudo service unifi_gate_opener restart"
                    ;;
                openbsd|netbsd)
                    echo "  Start:   sudo /etc/rc.d/unifi_gate_opener start"
                    echo "  Stop:    sudo /etc/rc.d/unifi_gate_opener stop"
                    echo "  Restart: sudo /etc/rc.d/unifi_gate_opener restart"
                    ;;
            esac
            ;;
    esac
    echo
    print_info "Configuration files:"
    echo "  Config:   $CONFIG_FILE"
    echo "  Database: $DATABASE_FILE"
    echo "  Logs:     $LOG_DIR"
    echo
    print_info "For support and documentation, visit:"
    echo "  https://github.com/$GITHUB_REPO"
    echo
}

# Main installation flow
main() {
    print_header
    
    check_root
    detect_system
    get_latest_release
    download_binary
    create_user
    create_directories
    configure_app
    install_service
    start_service
    check_service
    print_completion
}

# Run main function
main "$@"