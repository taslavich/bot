#!/bin/bash
cd /root/RTB/bot/twinbid-telegram-bot-admin-payments/cmd/bot

export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin

echo "$(date): Building bot application..."
go build -o main

if [ ! -f ./main ]; then
    echo "$(date): ERROR - Build failed"
    exit 1
fi

echo "$(date): Build completed successfully, restarting service..."
systemctl restart twinbidBot.service
sleep 1
systemctl status twinbidBot.service