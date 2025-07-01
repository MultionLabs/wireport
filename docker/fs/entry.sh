#!/bin/sh

# avoid overwriting resolv.conf by other components
echo "resolvconf=NO" >> /etc/resolvconf.conf

if [ "$1" = "gateway" ]; then
    # gateway mode

    # configure wireport gateway
    echo "> Configuring wireport gateway"
    
    wireport gateway start --configure

    # update configuration permissions
    chmod 600 /etc/wireguard/wg0.conf
    chmod 600 /etc/caddy/Caddyfile

    mv /etc/service/iptables-server /etc/service-disabled/
    mv /etc/service/wireport-server /etc/service-disabled/
elif [ "$1" = "join" ]; then
    # server
    echo "> Joining wireport network as server"

    # disable some services
    mv /etc/service/caddy /etc/service-disabled/
    mv /etc/service/wireport-gateway /etc/service-disabled/
    mv /etc/service/iptables-gateway /etc/service-disabled/
    
    wireport join "$2"
elif [ "$1" = "server" ]; then
    if [ "$2" = "start" ]; then
        echo "> Starting wireport server"

        mv /etc/service/caddy /etc/service-disabled/
        mv /etc/service/wireport-gateway /etc/service-disabled/
        mv /etc/service/iptables-gateway /etc/service-disabled/
    elif [ "$2" = "down" ]; then
        echo "> Tearing down wireport server"

        wireport server down
        exit 0
    else
        echo "Invalid command. Use 'start' or 'down'."
        exit 1
    fi
else
    echo "Invalid command. Use 'gateway' or 'join <TOKEN>'."
    exit 1
fi

chmod +x /etc/service/*/run && chmod +x /etc/service/*/finish

/usr/sbin/runsvdir /etc/service
