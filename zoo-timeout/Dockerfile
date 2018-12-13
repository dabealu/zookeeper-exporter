FROM zookeeper:3.4

RUN apk add --no-cache iptables

# Add custom entrypoint to set iptables rules and then resume the original entrypoint script
ADD custom-entrypoint.sh /
RUN cat /docker-entrypoint.sh >> /custom-entrypoint.sh

# Use custom entrypoint with default command taken from upstream Dockerfile
ENTRYPOINT ["/custom-entrypoint.sh"]
CMD ["zkServer.sh", "start-foreground"]
