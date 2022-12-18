# echo-server

- `/` status 200 and `hello world`
- `/echo` echo full request back to body
- `/cpu` cpu info
- `/vmem` virtual memory info
- `/net` return network interface list

```
docker run --rm -d -p 8080:18080 zengxu/echo-server
```