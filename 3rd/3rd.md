系统依赖的第三方库信息如下:
|**序号**|**库名**|**下载路径**|**备注**|
|:----:|:-----|:---------|:-----|
| 01 | cjson | https://github.com/DaveGamble/cJSON.git | 暂无 |
| 02 | protobuf-c | https://github.com/protobuf-c/protobuf-c.git | ./configure --disable-protoc |
| 03 | openssl | https://github.com/openssl/openssl.git | 暂无 |
| 04 | zlib | https://github.com/madler/zlib.git | 暂无 |
| 05 | libcurl | https://github.com/curl/curl.git | 暂无 |

## 编译

依赖工具: `git`, `make`, `gcc/clang`；`cmake`(cJSON)；`autoconf automake libtool`(protobuf-c/curl)

```bash
# 仅编译 C 库 -> 3rd/install/
cd 3rd && ./build_c.sh

# 或同时拉 Go 依赖 + 编译 C 库
cd 3rd && ./download.sh
```

编译顺序: zlib -> openssl -> cjson -> protobuf-c -> curl

安装前缀: `3rd/install/`（头文件 `include/`，库文件 `lib/`）

