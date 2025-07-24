# ✅ ДИАГНОСТИЧЕСКИЙ СКРИПТ для Linux - Сохраните как check_ports.sh

#!/bin/bash

echo "Checking if exchange servers are running..."
echo

echo "Live Exchange Ports:"
ss -tuln | grep :40101 || echo "Port 40101: No listeners found"
ss -tuln | grep :40102 || echo "Port 40102: No listeners found"
ss -tuln | grep :40103 || echo "Port 40103: No listeners found"
echo

echo "Test Exchange Ports:"
ss -tuln | grep :50101 || echo "Port 50101: No listeners found"
ss -tuln | grep :50102 || echo "Port 50102: No listeners found" 
ss -tuln | grep :50103 || echo "Port 50103: No listeners found"
echo

echo "Trying to connect to test ports:"

# Function to test port connectivity
test_port() {
    if timeout 2 bash -c "</dev/tcp/localhost/$1" 2>/dev/null; then
        echo "Port $1: OPEN"
    else
        echo "Port $1: CLOSED"
    fi
}

test_port 50101
test_port 50102
test_port 50103
echo

echo "Alternative check with netcat (if available):"
if command -v nc >/dev/null 2>&1; then
    echo -n "Port 50101: "
    if nc -z localhost 50101 2>/dev/null; then echo "OPEN"; else echo "CLOSED"; fi
    echo -n "Port 50102: "
    if nc -z localhost 50102 2>/dev/null; then echo "OPEN"; else echo "CLOSED"; fi
    echo -n "Port 50103: "
    if nc -z localhost 50103 2>/dev/null; then echo "OPEN"; else echo "CLOSED"; fi
else
    echo "netcat (nc) not available"
fi
echo

echo "Process listening on these ports:"
echo "Live ports (40101-40103):"
lsof -i :40101 2>/dev/null || echo "  No process on 40101"
lsof -i :40102 2>/dev/null || echo "  No process on 40102" 
lsof -i :40103 2>/dev/null || echo "  No process on 40103"

echo "Test ports (50101-50103):"
lsof -i :50101 2>/dev/null || echo "  No process on 50101"
lsof -i :50102 2>/dev/null || echo "  No process on 50102"
lsof -i :50103 2>/dev/null || echo "  No process on 50103"

echo
read -p "Press Enter to continue..."