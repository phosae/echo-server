# echo-server
- `/` echo full request back to body, use the query parameter `code` to configure the response code, and use the query parameter `duration` to configure the handling duration. For example, `curl -i '127.1:8080/?code=500&duration=2s'`.
- `/hello` status 200 and `hello world`
- `/cpu` cpu info
- `/vmem` virtual memory info
- `/net` return network interface list

```
docker run --rm -d -p 8080:80 zengxu/echo-server
```