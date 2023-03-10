# echo-server
- `/` echo full request back to body
- `/hello` status 200 and `hello world`
- `/cpu` cpu info
- `/vmem` virtual memory info
- `/net` return network interface list

```
docker run --rm -d -p 18080:8080 zengxu/echo-server
```