#!/bin/bash


ufw allow 22/tcp
ufw allow 80/tcp
ufw allow 443/tcp
ufw allow 4060/tcp
ufw allow 51820/udp

ufw enable
ufw status
