# 目的
只为进入状态为非Running的容器

# 进程
1. 可以进程共享了
2. 怎么文件系统共享呢  通过共享卷吗？

# 专业的社区
```
kubectl debug -it ephemeral-demo --image=busybox:1.28 --target=ephemeral-demo  // 也只进程共享
```