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
elif [ "$1" = "join" ]; then
    # server
    echo "> Joining wireport network"

    # disable some services
    mv /etc/service/caddy /etc/service-disabled/
    mv /etc/service/wireport /etc/service-disabled/
    mv /etc/service/iptables-host /etc/service-disabled/
    
    wireport join "$2"
else
    echo "Invalid command. Use 'host' or 'join <TOKEN>'."
    exit 1
fi

chmod +x /etc/service/*/run && chmod +x /etc/service/*/finish

/usr/sbin/runsvdir /etc/service
