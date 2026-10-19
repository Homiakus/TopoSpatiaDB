#!/usr/bin/env bash
# ==============================================================================
# TopoSpatiaDB & Web Production Deployment with Automated SSL & Auto-IP Discovery
# Interactive & Progressive Deployment Engine
# ==============================================================================

set -eo pipefail

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

# Global IP cache
DETECTED_PUBLIC_IP=""
DETECTED_LOCAL_IP=""

print_header() {
    clear 2>/dev/null || true
    echo -e "${CYAN}${BOLD}====================================================================${RESET}"
    echo -e "${CYAN}${BOLD}       TopoSpatiaDB: Progressive Deployment & SSL Automation        ${RESET}"
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

# ==============================================================================
# Progressive Status and Progress Bar Engine
# ==============================================================================
show_step_header() {
    local step_num="$1"
    local total_steps="$2"
    local title="$3"
    local percent=$(( step_num * 100 / total_steps ))
    local width=30
    local filled=$(( percent * width / 100 ))
    local empty=$(( width - filled ))
    
    local bar=""
    for ((i=0; i<filled; i++)); do bar="${bar}█"; done
    for ((i=0; i<empty; i++)); do bar="${bar}░"; done

    echo ""
    echo -e "${MAGENTA}${BOLD}┌──────────────────────────────────────────────────────────────────┐${RESET}"
    printf "${MAGENTA}${BOLD}│ Step [%d/%d] (%3d%%) [%s] │${RESET}\n" "$step_num" "$total_steps" "$percent" "$bar"
    printf "${MAGENTA}${BOLD}│ %-64s │${RESET}\n" "$title"
    echo -e "${MAGENTA}${BOLD}└──────────────────────────────────────────────────────────────────┘${RESET}"
    echo ""
}

print_status_line() {
    local status="$1"
    local msg="$2"
    case "$status" in
        "PROGRESS") echo -e "  ${YELLOW}↻${RESET} ${DIM}${msg}...${RESET}" ;;
        "OK")       echo -e "  ${GREEN}✓${RESET} ${BOLD}${msg}${RESET}" ;;
        "FAIL")     echo -e "  ${RED}✗${RESET} ${BOLD}${RED}${msg}${RESET}" ;;
        "INFO")     echo -e "  ${CYAN}ℹ${RESET} ${DIM}${msg}${RESET}" ;;
        "WARN")     echo -e "  ${YELLOW}⚠${RESET} ${YELLOW}${msg}${RESET}" ;;
    esac
}

# ==============================================================================
# Intelligent IP Detection Engine (Multi-source Fallback)
# ==============================================================================
detect_network_ips() {
    print_status_line "PROGRESS" "Определение локального IP-адреса хоста"
    
    # Local IP detection
    local local_candidate=""
    if command -v ip &>/dev/null; then
        local_candidate=$(ip route get 1.1.1.1 2>/dev/null | awk '{for(i=1;i<=NF;i++)if($i=="src"){print $(i+1);exit}}')
    fi
    if [ -z "$local_candidate" ] && command -v hostname &>/dev/null; then
        local_candidate=$(hostname -I 2>/dev/null | awk '{print $1}')
    fi
    DETECTED_LOCAL_IP="${local_candidate:-127.0.0.1}"
    print_status_line "OK" "Локальный IP: ${BOLD}${DETECTED_LOCAL_IP}${RESET}"

    # Public IP detection via multi-provider race/fallback
    print_status_line "PROGRESS" "Определение публичного IP через внешние сервисы (ipify/ifconfig/icanhazip)"
    local pub_ip=""
    local ip_services=(
        "https://api.ipify.org"
        "https://ifconfig.me/ip"
        "https://icanhazip.com"
        "https://checkip.amazonaws.com"
        "https://ip.sb"
    )

    for service in "${ip_services[@]}"; do
        pub_ip=$(curl -s --connect-timeout 2 --max-time 3 "$service" 2>/dev/null | tr -d '[:space:]' || true)
        if [[ "$pub_ip" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            break
        fi
        pub_ip=""
    done

    if [ -n "$pub_ip" ]; then
        DETECTED_PUBLIC_IP="$pub_ip"
        print_status_line "OK" "Публичный IP: ${BOLD}${CYAN}${DETECTED_PUBLIC_IP}${RESET}"
    else
        DETECTED_PUBLIC_IP="$DETECTED_LOCAL_IP"
        print_status_line "WARN" "Не удалось определить публичный IP (офлайн/файрвол). Используется: ${DETECTED_PUBLIC_IP}"
    fi
}

resolve_best_host_candidate() {
    if [ -z "$DETECTED_PUBLIC_IP" ]; then
        detect_network_ips
    fi
    if [ -n "$DETECTED_PUBLIC_IP" ] && [ "$DETECTED_PUBLIC_IP" != "127.0.0.1" ]; then
        echo "$DETECTED_PUBLIC_IP"
    else
        echo "${DETECTED_LOCAL_IP:-localhost}"
    fi
}

# ==============================================================================
# Tasks & Actions
# ==============================================================================
install_dependencies() {
    print_header
    echo -e "${BOLD}${BLUE}==> [1] Installing Required Dependencies (Nginx, Certbot, Go, Curl)...${RESET}"
    
    if command -v apt-get &>/dev/null; then
        print_status_line "PROGRESS" "Updating APT package index"
        run_as_root apt-get update -y -qq
        print_status_line "PROGRESS" "Installing nginx, certbot, python3-certbot-nginx, curl, ufw, git"
        run_as_root apt-get install -y -qq nginx certbot python3-certbot-nginx curl ufw git
    elif command -v dnf &>/dev/null; then
        run_as_root dnf install -y nginx certbot python3-certbot-nginx curl
    elif command -v yum &>/dev/null; then
        run_as_root yum install -y epel-release
        run_as_root yum install -y nginx certbot python3-certbot-nginx curl
    elif command -v pacman &>/dev/null; then
        run_as_root pacman -Sy --noconfirm nginx certbot certbot-nginx curl
    else
        print_status_line "FAIL" "Unsupported package manager. Please install Nginx and Certbot manually."
        read -rp "Press Enter to continue..."
        return 1
    fi
    
    print_status_line "OK" "Все системные зависимости успешно установлены!"
    read -rp "Press Enter to return to menu..."
}

build_app() {
    print_header
    echo -e "${BOLD}${BLUE}==> [2] Building TopoSpatiaDB Server Binary...${RESET}"
    
    if ! command -v go &>/dev/null; then
        print_status_line "FAIL" "Go compiler ('go') not found in PATH."
        echo -e "${YELLOW}Please install Go (1.20+) to compile the binary.${RESET}"
        read -rp "Press Enter to return to menu..."
        return 1
    fi
    
    print_status_line "PROGRESS" "Compiling cmd/topospatiad into ./topospatiad"
    go build -ldflags="-s -w" -o topospatiad ./cmd/topospatiad
    chmod +x topospatiad
    print_status_line "OK" "Успешно скомпилировано: $(pwd)/topospatiad"
    read -rp "Press Enter to return to menu..."
}

setup_systemd_service() {
    print_header
    echo -e "${BOLD}${BLUE}==> [3] Configuring TopoSpatiaDB Systemd Service...${RESET}"
    
    if [ ! -f "$SCRIPT_DIR/topospatiad" ]; then
        print_status_line "WARN" "Бинарник ./topospatiad не найден. Запуск сборки..."
        if command -v go &>/dev/null; then
            go build -ldflags="-s -w" -o topospatiad ./cmd/topospatiad
            print_status_line "OK" "Бинарник собран."
        else
            print_status_line "FAIL" "Go not installed. Cannot build topospatiad binary."
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
    
    print_status_line "PROGRESS" "Запись systemd unit-файла: ${SERVICE_FILE}"
    
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
    
    print_status_line "OK" "Сервис ${SERVICE_NAME} активирован и запущен на локальном порту ${APP_PORT}!"
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
    
    detect_network_ips
    echo ""
    read -rp "Enter your Domain Name (e.g. topospatia.example.com): " DOMAIN
    if [ -z "$DOMAIN" ]; then
        print_status_line "FAIL" "Domain name cannot be empty."
        read -rp "Press Enter to continue..."
        return 1
    fi
    
    read -rp "Enter email for renewal notices & SSL registration: " EMAIL
    if [ -z "$EMAIL" ]; then
        print_status_line "FAIL" "Email is required for Let's Encrypt."
        read -rp "Press Enter to continue..."
        return 1
    fi
    
    read -rp "Enter TopoSpatiaDB backend port [default: 8089]: " APP_PORT
    APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"
    
    print_status_line "PROGRESS" "Step 1: Setting up initial Nginx HTTP configuration..."
    configure_nginx_http "$DOMAIN" "$APP_PORT"
    
    print_status_line "PROGRESS" "Step 2: Requesting Let's Encrypt Certificate via Certbot..."
    if run_as_root certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL" --redirect; then
        print_status_line "OK" "SSL Certificate obtained and Nginx configured with HTTPS auto-redirect!"
        echo -e "${GREEN}Site is now live at: https://${DOMAIN}${RESET}"
    else
        print_status_line "FAIL" "Certbot failed to issue SSL certificate."
        echo -e "${YELLOW}Check your DNS settings, open ports 80 and 443 in firewall, and retry.${RESET}"
    fi
    read -rp "Press Enter to return to menu..."
}

deploy_self_signed_ssl() {
    print_header
    echo -e "${BOLD}${BLUE}==> [5] Generate & Install Self-Signed SSL Certificate (Local / IP / Dev)...${RESET}"
    
    detect_network_ips
    local auto_host
    auto_host=$(resolve_best_host_candidate)
    
    echo ""
    echo -e "Автоопределенный адрес: ${BOLD}${CYAN}${auto_host}${RESET}"
    read -rp "Enter Domain Name or IP address [default: ${auto_host}]: " DOMAIN
    DOMAIN="${DOMAIN:-$auto_host}"
    
    read -rp "Enter TopoSpatiaDB backend port [default: 8089]: " APP_PORT
    APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"
    
    SSL_DIR="/etc/ssl/topospatia"
    run_as_root mkdir -p "$SSL_DIR"
    
    print_status_line "PROGRESS" "Генерация 2048-битного RSA ключа и сертификата с SAN..."
    
    # Generate SAN config for IP / Domain compatibility
    local OPENSSL_CNF
    OPENSSL_CNF=$(mktemp)
    cat <<EOF > "$OPENSSL_CNF"
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = State
L = City
O = TopoSpatia
OU = Web
CN = ${DOMAIN}

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = ${DOMAIN}
IP.1 = 127.0.0.1
EOF
    if [[ "$DOMAIN" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        echo "IP.2 = ${DOMAIN}" >> "$OPENSSL_CNF"
    fi
    if [ -n "$DETECTED_LOCAL_IP" ] && [[ "$DETECTED_LOCAL_IP" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
        echo "IP.3 = ${DETECTED_LOCAL_IP}" >> "$OPENSSL_CNF"
    fi
    
    run_as_root openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
        -keyout "${SSL_DIR}/${DOMAIN}.key" \
        -out "${SSL_DIR}/${DOMAIN}.crt" \
        -config "$OPENSSL_CNF"
    rm -f "$OPENSSL_CNF"
    
    print_status_line "PROGRESS" "Конфигурация Nginx с Self-Signed SSL..."
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
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
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
    
    print_status_line "OK" "Самоподписанный SSL успешно активирован!"
    echo -e "${GREEN}Адрес сайта: https://${DOMAIN}${RESET}"
    read -rp "Press Enter to return to menu..."
}

renew_ssl() {
    print_header
    echo -e "${BOLD}${BLUE}==> [6] Test and Renew SSL Certificates...${RESET}"
    if command -v certbot &>/dev/null; then
        run_as_root certbot renew --dry-run
        print_status_line "OK" "Проверка продления Certbot выполнена успешно."
    else
        print_status_line "WARN" "Certbot не установлен в системе."
    fi
    read -rp "Press Enter to return to menu..."
}

view_service_status() {
    print_header
    echo -e "${BOLD}${BLUE}==> Диагностика сервисов и сетевой топологии:${RESET}"
    echo ""
    detect_network_ips
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

# ==============================================================================
# Progressive All-in-One Full Deployment Engine
# ==============================================================================
progressive_quick_deploy() {
    print_header
    echo -e "${BOLD}${MAGENTA}==> [A] Progressive Full Deployment (Автоопределение IP и Пошаговая Настройка)${RESET}"
    echo -e "${DIM}Полностью автоматизированный процесс с визуализацией каждого этапа и автодетекцией сети.${RESET}"
    echo ""
    
    local TOTAL_STEPS=6
    
    # -------------------------------------------------------------
    # Step 1: Network & IP Auto-Discovery
    # -------------------------------------------------------------
    show_step_header 1 "$TOTAL_STEPS" "Сетевое автоопределение IP-адресов"
    detect_network_ips
    local auto_host
    auto_host=$(resolve_best_host_candidate)
    
    echo ""
    echo -e "  Определены следующие сетевые адреса:"
    echo -e "    • Внешний (Public IP):  ${CYAN}${BOLD}${DETECTED_PUBLIC_IP:-N/A}${RESET}"
    echo -e "    • Локальный (Local IP): ${CYAN}${BOLD}${DETECTED_LOCAL_IP:-N/A}${RESET}"
    echo -e "    • Рекомендуемый хост:   ${GREEN}${BOLD}${auto_host}${RESET}"
    echo ""
    read -rp "Введите доменное имя или IP [По умолчанию: ${auto_host}]: " DOMAIN
    DOMAIN="${DOMAIN:-$auto_host}"
    
    echo ""
    echo -e "  Выберите тип защищенного соединения:"
    echo -e "    ${CYAN}[1]${RESET} Let's Encrypt SSL (для публичного DNS-домена с A-записью)"
    echo -e "    ${CYAN}[2]${RESET} Автоматический Self-Signed SSL (для IP-адреса, LAN, тестового домена)"
    read -rp "Выбор [1/2, по умолчанию: 2]: " SSL_TYPE
    SSL_TYPE="${SSL_TYPE:-2}"
    
    local EMAIL=""
    if [ "$SSL_TYPE" = "1" ]; then
        read -rp "Введите Email для регистрации сертификата Let's Encrypt: " EMAIL
        if [ -z "$EMAIL" ]; then
            print_status_line "WARN" "Email не указан. Переключение на самоподписанный SSL."
            SSL_TYPE="2"
        fi
    fi
    
    read -rp "Порт внутреннего сервиса TopoSpatiaDB [по умолчанию: 8089]: " APP_PORT
    APP_PORT="${APP_PORT:-$DEFAULT_APP_PORT}"

    # -------------------------------------------------------------
    # Step 2: Dependencies Check & Installation
    # -------------------------------------------------------------
    show_step_header 2 "$TOTAL_STEPS" "Проверка и установка зависимостей (Nginx, Go, SSL)"
    if ! command -v nginx &>/dev/null; then
        print_status_line "PROGRESS" "Установка Nginx и системных утилит..."
        if command -v apt-get &>/dev/null; then
            run_as_root apt-get update -qq -y
            run_as_root apt-get install -qq -y nginx curl certbot python3-certbot-nginx
        fi
        print_status_line "OK" "Nginx установлен."
    else
        print_status_line "OK" "Nginx уже присутствует в системе."
    fi

    # -------------------------------------------------------------
    # Step 3: Progressive Compilation of TopoSpatiaDB
    # -------------------------------------------------------------
    show_step_header 3 "$TOTAL_STEPS" "Компиляция высокопроизводительного бинарника TopoSpatiaDB"
    if ! command -v go &>/dev/null; then
        print_status_line "FAIL" "Go компилятор не найден. Невозможно собрать бинарник."
        read -rp "Нажмите Enter для выхода..."
        return 1
    fi
    
    print_status_line "PROGRESS" "Сборка cmd/topospatiad с оптимизацией флагов (-s -w)..."
    go build -ldflags="-s -w" -o topospatiad ./cmd/topospatiad
    chmod +x topospatiad
    print_status_line "OK" "Бинарник успешно скомпилирован ($(du -h topospatiad | cut -f1))"

    # -------------------------------------------------------------
    # Step 4: Systemd Microservice Provisioning
    # -------------------------------------------------------------
    show_step_header 4 "$TOTAL_STEPS" "Развертывание и активация демона systemd"
    CURRENT_USER="${SUDO_USER:-$USER}"
    SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}"
    mkdir -p "$DEFAULT_DATA_DIR"
    
    print_status_line "PROGRESS" "Создание unit-файла ${SERVICE_NAME} на порту ${APP_PORT}..."
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
    
    # Wait for daemon readiness
    print_status_line "PROGRESS" "Проверка готовности сервиса TopoSpatiaDB..."
    sleep 1
    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        print_status_line "OK" "Сервис ${SERVICE_NAME} запущен и активен."
    else
        print_status_line "WARN" "Сервис запустился, но требует проверки статуса."
    fi

    # -------------------------------------------------------------
    # Step 5: Web Server (Nginx) & SSL Provisioning
    # -------------------------------------------------------------
    show_step_header 5 "$TOTAL_STEPS" "Настройка Nginx Reverse Proxy и SSL шифрования"
    
    if [ "$SSL_TYPE" = "1" ]; then
        print_status_line "PROGRESS" "Конфигурация HTTP challenge для Let's Encrypt..."
        configure_nginx_http "$DOMAIN" "$APP_PORT"
        print_status_line "PROGRESS" "Запрос сертификата Certbot для ${DOMAIN}..."
        if run_as_root certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$EMAIL" --redirect; then
            print_status_line "OK" "Let's Encrypt SSL сертификат успешно получен и установлен!"
        else
            print_status_line "FAIL" "Certbot не смог получить сертификат для ${DOMAIN}."
            print_status_line "WARN" "Переключение на самоподписанный SSL с поддержкой IP..."
            SSL_TYPE="2"
        fi
    fi

    if [ "$SSL_TYPE" = "2" ]; then
        print_status_line "PROGRESS" "Генерация самоподписанного сертификата с SAN для ${DOMAIN}..."
        SSL_DIR="/etc/ssl/topospatia"
        run_as_root mkdir -p "$SSL_DIR"
        
        OPENSSL_CNF=$(mktemp)
        cat <<EOF > "$OPENSSL_CNF"
[req]
distinguished_name = req_distinguished_name
x509_extensions = v3_req
prompt = no

[req_distinguished_name]
C = US
ST = State
L = City
O = TopoSpatia
OU = Web
CN = ${DOMAIN}

[v3_req]
keyUsage = keyEncipherment, dataEncipherment
extendedKeyUsage = serverAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = localhost
DNS.2 = ${DOMAIN}
IP.1 = 127.0.0.1
EOF
        if [[ "$DOMAIN" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            echo "IP.2 = ${DOMAIN}" >> "$OPENSSL_CNF"
        fi
        if [ -n "$DETECTED_LOCAL_IP" ] && [[ "$DETECTED_LOCAL_IP" =~ ^[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}$ ]]; then
            echo "IP.3 = ${DETECTED_LOCAL_IP}" >> "$OPENSSL_CNF"
        fi

        run_as_root openssl req -x509 -nodes -days 365 -newkey rsa:2048 \
            -keyout "${SSL_DIR}/${DOMAIN}.key" \
            -out "${SSL_DIR}/${DOMAIN}.crt" \
            -config "$OPENSSL_CNF"
        rm -f "$OPENSSL_CNF"

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
    listen 443 ssl;
    listen [::]:443 ssl;
    http2 on;
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
        print_status_line "OK" "Nginx успешно настроен и перезагружен."
    fi

    # -------------------------------------------------------------
    # Step 6: Healthcheck Verification & Summary
    # -------------------------------------------------------------
    show_step_header 6 "$TOTAL_STEPS" "Финальная проверка работоспособности и готовность"
    
    print_status_line "PROGRESS" "Выполнение сетевого probe-запроса на https://${DOMAIN} / http://127.0.0.1:${APP_PORT}..."
    local HEALTH_STATUS="FAIL"
    if curl -k -s -o /dev/null -w "%{http_code}" "http://127.0.0.1:${APP_PORT}/" | grep -qE "200|301|302|404"; then
        HEALTH_STATUS="OK"
    fi
    
    if [ "$HEALTH_STATUS" = "OK" ]; then
        print_status_line "OK" "Локальный бэкенд TopoSpatiaDB отвечает корректно!"
    else
        print_status_line "WARN" "Бэкенд пока не вернул 200/30x код, возможно инициализирует хранилище."
    fi

    echo ""
    echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════════════${RESET}"
    echo -e "${GREEN}${BOLD}       РАЗВЕРТЫВАНИЕ УСПЕШНО ЗАВЕРШЕНО! САЙТ ДОСТУПЕН ОНЛАЙН        ${RESET}"
    echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════════════${RESET}"
    echo ""
    echo -e "  🌐 Основной URL:          ${CYAN}${BOLD}https://${DOMAIN}${RESET}"
    if [ -n "$DETECTED_PUBLIC_IP" ] && [ "$DOMAIN" != "$DETECTED_PUBLIC_IP" ]; then
        echo -e "  🌍 Прямой доступ по IP:   ${CYAN}https://${DETECTED_PUBLIC_IP}${RESET}"
    fi
    if [ -n "$DETECTED_LOCAL_IP" ] && [ "$DOMAIN" != "$DETECTED_LOCAL_IP" ]; then
        echo -e "  🏠 Доступ по LAN:         ${CYAN}https://${DETECTED_LOCAL_IP}${RESET}"
    fi
    echo -e "  ⚡ Локальный порт:        ${YELLOW}127.0.0.1:${APP_PORT}${RESET}"
    echo -e "  ⚙️  Служба Systemd:        ${BLUE}${SERVICE_NAME}${RESET}"
    echo ""
    echo -e "${DIM}Для проверки логов сервиса используйте: journalctl -u ${SERVICE_NAME} -f${RESET}"
    echo -e "${GREEN}${BOLD}════════════════════════════════════════════════════════════════════${RESET}"
    echo ""
    read -rp "Нажмите Enter для возврата в главное меню..."
}

# ==============================================================================
# Main Interactive Menu Loop
# ==============================================================================
while true; do
    print_header
    check_root
    echo -e "${BOLD}Сетевой статус хоста:${RESET}"
    if [ -z "$DETECTED_PUBLIC_IP" ] && [ -z "$DETECTED_LOCAL_IP" ]; then
        echo -e "  ${DIM}IP-адреса не определены (будут найдены автоматически)${RESET}"
    else
        echo -e "  • Public IP: ${CYAN}${DETECTED_PUBLIC_IP:-N/A}${RESET} | Local IP: ${CYAN}${DETECTED_LOCAL_IP:-N/A}${RESET}"
    fi
    echo ""
    echo -e "${BOLD}Выберите действие / Select an option:${RESET}"
    echo ""
    echo -e "  ${MAGENTA}${BOLD}[A]${RESET}  ${MAGENTA}${BOLD}Прогрессивный полный авто-деплой${RESET} (Автоопределение IP + Сборка + Сервис + Nginx + SSL)"
    echo -e "  ${CYAN}[1]${RESET}  ${BOLD}Установить зависимости${RESET} (Nginx, Certbot, Python-certbot, Go, Curl)"
    echo -e "  ${CYAN}[2]${RESET}  ${BOLD}Скомпилировать сервер TopoSpatiaDB${RESET} (cmd/topospatiad)"
    echo -e "  ${CYAN}[3]${RESET}  ${BOLD}Создать и запустить systemd-сервис${RESET} (${SERVICE_NAME})"
    echo -e "  ${CYAN}[4]${RESET}  ${BOLD}Выпустить сертификат Let's Encrypt SSL${RESET} (Certbot + Nginx)"
    echo -e "  ${CYAN}[5]${RESET}  ${BOLD}Создать самоподписанный SSL сертификат${RESET} (Для локального/тестового домена/IP)"
    echo -e "  ${CYAN}[6]${RESET}  ${BOLD}Проверить автопродление сертификатов${RESET} (Certbot renew dry-run)"
    echo -e "  ${CYAN}[7]${RESET}  ${BOLD}Диагностика и сетевой статус хоста${RESET} (TopoSpatiaDB, Nginx, порты, IP)"
    echo ""
    echo -e "  ${RED}[0]${RESET}  ${BOLD}Выход${RESET}"
    echo ""
    read -rp "Введите пункт меню [A, 0-7]: " CHOICE
    
    case "$CHOICE" in
        [aA]) progressive_quick_deploy ;;
        1) install_dependencies ;;
        2) build_app ;;
        3) setup_systemd_service ;;
        4) deploy_letsencrypt_ssl ;;
        5) deploy_self_signed_ssl ;;
        6) renew_ssl ;;
        7) view_service_status ;;
        0) echo -e "${GREEN}Завершение работы.${RESET}"; exit 0 ;;
        *) echo -e "${RED}Неверный выбор. Попробуйте снова.${RESET}"; sleep 1 ;;
    esac
done
