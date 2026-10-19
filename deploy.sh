#!/usr/bin/env bash
# ==============================================================================
# TopoSpatiaDB & Web Production Deployment with Automated SSL (Certbot / Self-signed)
# Interactive Management & Deployment Console
# ==============================================================================

set -e

# Styling & Colors
BOLD='\033[1m'
DIM='\033[2m'
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
RESET='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

NGINX_CONF_DIR="/etc/nginx"
APP_NAME="topospatia"
SERVICE_NAME="topospatia.service"
DEFAULT_APP_PORT="8089"
DEFAULT_DATA_DIR="$HOME/.topospatia_data"

print_header() {
    clear 2>/dev/null || true
    echo -e "${CYAN}${BOLD}====================================================================${RESET}"
    echo -e "${CYAN}${BOLD}       TopoSpatiaDB: Production Deployment & SSL Automation        ${RESET}"
    echo -e "${DIM}       Sheaf Holographic Topological DBMS Web Platform & SSL Setup  ${RESET}"
    echo -e "${CYAN}${BOLD}====================================================================${RESET}"
    echo ""
}

check_root() {
    if [ "$EUID" -ne 0 ]; then
        echo -e "${YELLOW}[!] Note: Administrative actions (nginx, systemd, certbot) require sudo.${RESET}"
    fi
}

run_as_root() {
    if [ "$EUID" -eq 0 ]; then
        "$@"
    else
        sudo "$@"
    fi
}

install_dependencies() {
    print_header
    echo -e "${BOLD}${BLUE}==> [1] Installing Required Dependencies (Nginx, Certbot, Go, Curl)...${RESET}"
    
    if command -v apt-get &>/dev/null; then
        echo -e "${DIM}Updating APT package lists...${RESET}"
        run_as_root apt-get update -y
        run_as_root apt-get install -y nginx certbot python3-certbot-nginx curl ufw git
    elif command -v dnf &>/dev/null; then
        run_as_root dnf install -y nginx certbot python3-certbot-nginx curl
    elif command -v yum &>/dev/null; then
        run_as_root yum install -y epel-release
        run_as_root yum install -y nginx certbot python3-certbot-nginx curl
    elif command -v pacman &>/dev/null; then
        run_as_root pacman -Sy --noconfirm nginx certbot certbot-nginx curl
    else
        echo -e "${RED}[ERROR] Unsupported package manager. Please install Nginx and Certbot manually.${RESET}"
        read -rp "Press Enter to continue..."
        return 1
    fi
    
    echo -e "${GREEN}[OK] Core packages installed successfully!${RESET}"
    read -rp "Press Enter to return to menu..."
}

build_app() {
    print_header
    echo -e "${BOLD}${BLUE}==> [2] Building TopoSpatiaDB Server Binary...${RESET}"
    
    if ! command -v go &>/dev/null; then
        echo -e "${RED}[ERROR] Go compiler ('go') not found in PATH.${RESET}"
        echo -e "${YELLOW}Please install Go (1.20+) to compile the binary.${RESET}"
        read -rp "Press Enter to return to menu..."
        return 1
    fi
    
    echo -e "${DIM}Compiling cmd/topospatiad into ./topospatiad...${RESET}"
    go build -ldflags="-s -w" -o topospatiad ./cmd/topospatiad
    chmod +x topospatiad
    echo -e "${GREEN}[OK] Successfully compiled: $(pwd)/topospatiad${RESET}"
    read -rp "Press Enter to return to menu..."
}

setup_systemd_service() {
    print_header
    echo -e "${BOLD}${BLUE}==> [3] Configuring TopoSpatiaDB Systemd Service...${RESET}"
    
    if [ ! -f "$SCRIPT_DIR/topospatiad" ]; then
        echo -e "${YELLOW}[!] Binary ./topospatiad not found. Compiling first...${RESET}"
        if command -v go &>/dev/null; then
            go build -ldflags="-s -w" -o topospatiad ./cmd/topospatiad
        else
            echo -e "${RED}[ERROR] Go not installed. Cannot build topospatiad binary.${RESET}"
            read -rp "Press Enter to return to menu..."
            return 1
        fi
    fi
    
    read -rp "Enter listening port for internal backend [default: 8089]: " APP_PORT
    APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"
    
    read -rp "Enter data storage directory [default: $DEFAULT_DATA_DIR]: " APP_DATA
    APP_DATA="${APP_DATA:-$DEFAULT_DATA_DIR}"
    mkdir -p "$APP_DATA"
    
    CURRENT_USER="${SUDO_USER:-$USER}"
    SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}"
    
    echo -e "${DIM}Writing service definition to ${SERVICE_FILE}...${RESET}"
    
    TMP_SVC=$(mktemp)
    cat <<EOF > "$TMP_SVC"
[Unit]
Description=TopoSpatiaDB Topological Multidimensional Holographic DBMS & Web
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${SCRIPT_DIR}
Environment="PORT=${APP_PORT}"
Environment="DATA_DIR=${APP_DATA}"
ExecStart=${SCRIPT_DIR}/topospatiad
Restart=always
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF

    run_as_root mv "$TMP_SVC" "$SERVICE_FILE"
    run_as_root systemctl daemon-reload
    run_as_root systemctl enable "${SERVICE_NAME}"
    run_as_root systemctl restart "${SERVICE_NAME}"
    
    echo -e "${GREEN}[OK] Service ${SERVICE_NAME} installed and running on internal port ${APP_PORT}!${RESET}"
    read -rp "Press Enter to return to menu..."
}

configure_nginx_http() {
    local DOMAIN="$1"
    local PORT="$2"
    
    CONF_FILE="/etc/nginx/sites-available/${DOMAIN}.conf"
    if [ ! -d "/etc/nginx/sites-available" ]; then
        CONF_FILE="/etc/nginx/conf.d/${DOMAIN}.conf"
    fi
    
    TMP_CONF=$(mktemp)
    cat <<EOF > "$TMP_CONF"
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};

    location /.well-known/acme-challenge/ {
        root /var/www/html;
    }

    location / {
        proxy_pass http://127.0.0.1:${PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
    run_as_root mkdir -p /var/www/html
    run_as_root mv "$TMP_CONF" "$CONF_FILE"
    if [ -d "/etc/nginx/sites-enabled" ]; then
        run_as_root ln -sf "$CONF_FILE" "/etc/nginx/sites-enabled/${DOMAIN}.conf"
    fi
    run_as_root nginx -t
    run_as_root systemctl reload nginx
}

deploy_letsencrypt_ssl() {
    print_header
    echo -e "${BOLD}${BLUE}==> [4] Issue Let's Encrypt Free SSL Certificate (Certbot)...${RESET}"
    echo -e "${YELLOW}[Notice] Your domain must point (DNS A/AAAA record) to this server's public IP!${RESET}"
    echo ""
    
    read -rp "Enter your Domain Name (e.g. topospatia.example.com): " DOMAIN
    if [ -z "$DOMAIN" ]; then
        echo -e "${RED}[ERROR] Domain name cannot be empty.${RESET}"
        read -rp "Press Enter to continue..."
        return 1
    fi
    
    read -rp "Enter email for renewal notices & SSL registration: " EMAIL
    if [ -z "$EMAIL" ]; then
        echo -e "${RED}[ERROR] Email is required for Let's Encrypt.${RESET}"
        read -rp "Press Enter to continue..."
        return 1
    fi
    
    read -rp "Enter TopoSpatiaDB backend port [default: 8089]: " APP_PORT
    APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"
    
    echo -e "${DIM}Step 1: Setting up initial Nginx HTTP configuration...${RESET}"
    configure_nginx_http "$DOMAIN" "$APP_PORT"
    
    echo -e "${DIM}Step 2: Requesting Let's Encrypt Certificate via Certbot...${RESET}"
    if run_as_root certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL" --redirect; then
        echo -e "${GREEN}${BOLD}[SUCCESS] SSL Certificate obtained and Nginx configured with HTTPS auto-redirect!${RESET}"
        echo -e "${GREEN}Site is now live at: https://${DOMAIN}${RESET}"
    else
        echo -e "${RED}[FAILED] Certbot failed to issue SSL certificate.${RESET}"
        echo -e "${YELLOW}Check your DNS settings, open ports 80 and 443 in firewall, and retry.${RESET}"
    fi
    read -rp "Press Enter to return to menu..."
}

deploy_self_signed_ssl() {
    print_header
    echo -e "${BOLD}${BLUE}==> [5] Generate & Install Self-Signed SSL Certificate (Local / Testing)...${RESET}"
    echo -e "${DIM}Ideal for local testing, LAN, dev domains or IP addresses.${RESET}"
    echo ""
    
    read -rp "Enter Domain Name or IP address [default: localhost]: " DOMAIN
    DOMAIN="${DOMAIN:-localhost}"
    
    read -rp "Enter TopoSpatiaDB backend port [default: 8089]: " APP_PORT
    APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"
    
    SSL_DIR="/etc/ssl/topospatia"
    run_as_root mkdir -p "$SSL_DIR"
    
    echo -e "${DIM}Generating 2048-bit RSA private key and self-signed certificate (365 days)...${RESET}"
    run_as_root openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout "${SSL_DIR}/${DOMAIN}.key" \
        -out "${SSL_DIR}/${DOMAIN}.crt" \
        -subj "/C=US/ST=State/L=City/O=TopoSpatia/OU=Web/CN=${DOMAIN}"
    
    echo -e "${DIM}Configuring Nginx with Self-Signed SSL for ${DOMAIN}...${RESET}"
    CONF_FILE="/etc/nginx/sites-available/${DOMAIN}.conf"
    if [ ! -d "/etc/nginx/sites-available" ]; then
        CONF_FILE="/etc/nginx/conf.d/${DOMAIN}.conf"
    fi
    
    TMP_CONF=$(mktemp)
    cat <<EOF > "$TMP_CONF"
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${DOMAIN};

    ssl_certificate ${SSL_DIR}/${DOMAIN}.crt;
    ssl_certificate_key ${SSL_DIR}/${DOMAIN}.key;

    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_ciphers HIGH:!aNULL:!MD5;
    ssl_prefer_server_ciphers on;

    location / {
        proxy_pass http://127.0.0.1:${APP_PORT};
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
    run_as_root mv "$TMP_CONF" "$CONF_FILE"
    if [ -d "/etc/nginx/sites-enabled" ]; then
        run_as_root ln -sf "$CONF_FILE" "/etc/nginx/sites-enabled/${DOMAIN}.conf"
    fi
    run_as_root nginx -t
    run_as_root systemctl reload nginx
    
    echo -e "${GREEN}${BOLD}[SUCCESS] Self-signed SSL active!${RESET}"
    echo -e "${GREEN}Access site at: https://${DOMAIN}${RESET}"
    read -rp "Press Enter to return to menu..."
}

renew_ssl() {
    print_header
    echo -e "${BOLD}${BLUE}==> [6] Test and Renew SSL Certificates...${RESET}"
    echo -e "${DIM}Running Certbot renewal dry-run / force renew...${RESET}"
    
    if command -v certbot &>/dev/null; then
        run_as_root certbot renew --dry-run
        echo -e "${GREEN}[OK] Certbot renewal check completed.${RESET}"
    else
        echo -e "${YELLOW}[!] Certbot is not installed.${RESET}"
    fi
    read -rp "Press Enter to return to menu..."
}

view_service_status() {
    print_header
    echo -e "${BOLD}${BLUE}==> Service & Network Health Diagnostics:${RESET}"
    echo ""
    echo -e "${CYAN}--- TopoSpatiaDB Service Status ---${RESET}"
    systemctl status "${SERVICE_NAME}" --no-pager || true
    echo ""
    echo -e "${CYAN}--- Nginx Status ---${RESET}"
    systemctl status nginx --no-pager || true
    echo ""
    echo -e "${CYAN}--- Open Ports (80, 443, 8089) ---${RESET}"
    if command -v ss &>/dev/null; then
        ss -tuln | grep -E ':(80|443|8089)' || true
    fi
    echo ""
    read -rp "Press Enter to return to menu..."
}

full_quick_deploy() {
    print_header
    echo -e "${BOLD}${MAGENTA}==> [A] All-in-One Automated Full Deployment${RESET}"
    echo -e "${DIM}This will build the app, create systemd service, configure Nginx, and issue SSL.${RESET}"
    echo ""
    
    read -rp "Enter Domain Name (e.g. topospatia.example.com or localhost): " DOMAIN
    DOMAIN="${DOMAIN:-localhost}"
    
    read -rp "Do you want: (1) Let's Encrypt (Public Domain) or (2) Self-Signed SSL (Dev/Local) [1/2]: " SSL_TYPE
    
    # 1. Build
    echo -e "\n${BLUE}Step 1: Building TopoSpatiaDB...${RESET}"
    if command -v go &>/dev/null; then
        go build -ldflags="-s -w" -o topospatiad ./cmd/topospatiad
    fi
    
    # 2. Service
    echo -e "\n${BLUE}Step 2: Setting up Systemd Service...${RESET}"
    CURRENT_USER="${SUDO_USER:-$USER}"
    SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}"
    TMP_SVC=$(mktemp)
    cat <<EOF > "$TMP_SVC"
[Unit]
Description=TopoSpatiaDB Topological Multidimensional Holographic DBMS & Web
After=network.target

[Service]
Type=simple
User=${CURRENT_USER}
WorkingDirectory=${SCRIPT_DIR}
Environment="PORT=8089"
Environment="DATA_DIR=${DEFAULT_DATA_DIR}"
ExecStart=${SCRIPT_DIR}/topospatiad
Restart=always
RestartSec=3
LimitNOFILE=65536

[Install]
WantedBy=multi-user.target
EOF
    run_as_root mv "$TMP_SVC" "$SERVICE_FILE"
    run_as_root systemctl daemon-reload
    run_as_root systemctl enable --now "${SERVICE_NAME}"
    
    # 3. SSL & Nginx
    if [ "$SSL_TYPE" = "1" ]; then
        read -rp "Enter email for Let's Encrypt: " EMAIL
        configure_nginx_http "$DOMAIN" "8089"
        run_as_root certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL" --redirect
    else
        SSL_DIR="/etc/ssl/topospatia"
        run_as_root mkdir -p "$SSL_DIR"
        run_as_root openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout "${SSL_DIR}/${DOMAIN}.key" \
            -out "${SSL_DIR}/${DOMAIN}.crt" \
            -subj "/C=US/ST=State/L=City/O=TopoSpatia/OU=Web/CN=${DOMAIN}"
        
        CONF_FILE="/etc/nginx/sites-available/${DOMAIN}.conf"
        if [ ! -d "/etc/nginx/sites-available" ]; then
            CONF_FILE="/etc/nginx/conf.d/${DOMAIN}.conf"
        fi
        
        TMP_CONF=$(mktemp)
        cat <<EOF > "$TMP_CONF"
server {
    listen 80;
    listen [::]:80;
    server_name ${DOMAIN};
    return 301 https://\$host\$request_uri;
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name ${DOMAIN};

    ssl_certificate ${SSL_DIR}/${DOMAIN}.crt;
    ssl_certificate_key ${SSL_DIR}/${DOMAIN}.key;

    location / {
        proxy_pass http://127.0.0.1:8089;
        proxy_http_version 1.1;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection 'upgrade';
        proxy_set_header Host \$host;
        proxy_cache_bypass \$http_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }
}
EOF
        run_as_root mv "$TMP_CONF" "$CONF_FILE"
        if [ -d "/etc/nginx/sites-enabled" ]; then
            run_as_root ln -sf "$CONF_FILE" "/etc/nginx/sites-enabled/${DOMAIN}.conf"
        fi
        run_as_root nginx -t
        run_as_root systemctl reload nginx
    fi
    
    echo -e "\n${GREEN}${BOLD}====================================================================${RESET}"
    echo -e "${GREEN}${BOLD}   Deployment Completed Successfully! Site is live over HTTPS!       ${RESET}"
    echo -e "${GREEN}   URL: https://${DOMAIN}${RESET}"
    echo -e "${GREEN}${BOLD}====================================================================${RESET}"
    read -rp "Press Enter to return to menu..."
}

# ==============================================================================
# Main Interactive Menu Loop
# ==============================================================================
while true; do
    print_header
    check_root
    echo -e "${BOLD}Выберите действие / Select an option:${RESET}"
    echo ""
    echo -e "  ${CYAN}[1]${RESET}  ${BOLD}Установить зависимости${RESET} (Nginx, Certbot, Python-certbot, Go, Curl)"
    echo -e "  ${CYAN}[2]${RESET}  ${BOLD}Скомпилировать сервер TopoSpatiaDB${RESET} (cmd/topospatiad)"
    echo -e "  ${CYAN}[3]${RESET}  ${BOLD}Создать и запустить systemd-сервис${RESET} (${SERVICE_NAME})"
    echo -e "  ${CYAN}[4]${RESET}  ${BOLD}Выпустить сертификат Let's Encrypt SSL${RESET} (Certbot + Nginx)"
    echo -e "  ${CYAN}[5]${RESET}  ${BOLD}Создать самоподписанный SSL сертификат${RESET} (Для локального/тестового домена/IP)"
    echo -e "  ${CYAN}[6]${RESET}  ${BOLD}Проверить автопродление сертификатов${RESET} (Certbot renew dry-run)"
    echo -e "  ${CYAN}[7]${RESET}  ${BOLD}Диагностика и статус сервисов${RESET} (TopoSpatiaDB, Nginx, порты)"
    echo ""
    echo -e "  ${MAGENTA}[A]${RESET}  ${BOLD}Полный авто-деплой 'всё в одном'${RESET} (Сборка + Сервис + Nginx + SSL)"
    echo ""
    echo -e "  ${RED}[0]${RESET}  ${BOLD}Выход${RESET}"
    echo ""
    read -rp "Введите пункт меню [0-7, A]: " CHOICE
    
    case "$CHOICE" in
        1) install_dependencies ;;
        2) build_app ;;
        3) setup_systemd_service ;;
        4) deploy_letsencrypt_ssl ;;
        5) deploy_self_signed_ssl ;;
        6) renew_ssl ;;
        7) view_service_status ;;
        [aA]) full_quick_deploy ;;
        0) echo -e "${GREEN}Завершение работы.${RESET}"; exit 0 ;;
        *) echo -e "${RED}Неверный выбор. Попробуйте снова.${RESET}"; sleep 1 ;;
    esac
done
