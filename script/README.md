### exportyaml.sh
```shell
./exportyaml.sh nsname  # 导出nsname命名空间下的yaml文件
```

### mig_public.sh
```shell
mongodb数据 全量+增量数据同步
1. 配合定是任务使用，需要修改脚本中 `TMP_FILE=/data1/public/` 该目录为临时文件目录
*/30 * * * * /data1/public/mig_public.sh >> /root/mig_public.log 2>&1 &   # 每30分钟执行一次脚本
```


