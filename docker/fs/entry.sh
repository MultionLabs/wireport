#!/bin/sh

# avoid overwriting resolv.conf by other components
echo "resolvconf=NO" >> /etc/resolvconf.conf

if [ "$1" = "host" ]; then
    # host mode

    # configure wireport host
    echo "> Configuring wireport host"
    
    wireport host start --configure

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
    mv /etc/service/wireport-host /etc/service-disabled/
    mv /etc/service/iptables-host /etc/service-disabled/
    
    wireport join "$2"
elif [ "$1" = "server" ]; then
    if [ "$2" = "start" ]; then
        echo "> Starting wireport server"

        mv /etc/service/wireport-host /etc/service-disabled/
    elif [ "$2" = "disconnect" ]; then
        echo "> Disconnecting wireport server"

        wireport server disconnect
        exit 0
    else
        echo "Invalid command. Use 'start' or 'disconnect'."
        exit 1
    fi
else
    echo "Invalid command. Use 'host' or 'join <TOKEN>'."
    exit 1
fi

chmod +x /etc/service/*/run && chmod +x /etc/service/*/finish

/usr/sbin/runsvdir /etc/service
