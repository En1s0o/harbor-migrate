# 使用说明



## 编译

```shell
go build
```



## 运行

> 示例：把 https://pcr.io 迁移到 https://3.pcr.io

```shell
./harbor-migrate \
--source-url https://pcr.io \
--source-user admin \
--source-pass Harbor12345 \
--target-url https://3.pcr.io \
--target-user admin \
--target-pass Harbor12345
```

