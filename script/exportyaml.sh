#!/bin/bash

NAMESPACE=$1

if [[ -z $NAMESPACE ]]; then
    #statements
    echo "未指定 namespace, 请使用 kubectl get ns 获取 namespace 清单，并选择其中一个 namespace"
    echo "示例: $0 ops"
    exit 0
else
    echo "开始从 ${NAMESPACE} 导出数据"
fi

type jq > /dev/null 2>&1
if [[ $? != 0 ]]; then
    yum -y install jq
    [[ $? != 0 ]] && echo "[Error] Install jq failed." && exit 1
fi

# cluster全局资源只需要导出一次，当namespace 为default 时执行
if [[ ${NAMESPACE} = "default" ]]; then
    rm -rf  ${RESOURCE_KIND} && mkdir -p ${RESOURCE_KIND}
    for n in $(kubectl get -o=name clusterrole,clusterrolebinding); do
        RESOURCE_KIND=$(dirname $n)
        RESOURCE_KIND=${RESOURCE_KIND/.*}
        RESOURCE_NAME=${n/$(dirname $n)\/}

        echo "kubectl get ${RESOURCE_KIND} ${RESOURCE_NAME}"
        kubectl get ${RESOURCE_KIND} ${RESOURCE_NAME} -o json | jq --sort-keys \
            'del(
                .metadata.creationTimestamp,
                .metadata.resourceVersion,
                .metadata.selfLink,
                .metadata.uid,
                .status
            )' | python -c 'import sys, yaml, json; yaml.safe_dump(json.load(sys.stdin), sys.stdout, default_flow_style=False)' > "${RESOURCE_KIND}/${RESOURCE_NAME}.yaml"
    done
fi


# 导出其他 namespace 级别资源
rm -rf ${NAMESPACE}/${RESOURCE_KIND}
for i in service deployment statefulset daemonset configmap ingress pv pvc hpa job cronjob serviceaccount secret role rolebinding;do
   mkdir -p ${NAMESPACE}/$i
done 


for n in $(kubectl -n ${NAMESPACE} get -o=name service,deployment,statefulset,daemonset,configmap,ingress,pv,pvc,hpa,job,cronjob,serviceaccount,secret,role,rolebinding); do
    RESOURCE_KIND=$(dirname $n)
    RESOURCE_KIND=${RESOURCE_KIND/.*}
    RESOURCE_NAME=${n/$(dirname $n)\/}

    # Secret has some bug when using --export, and clusterrolebindings clusterroles not support --export
    echo "kubectl -n ${NAMESPACE} get ${RESOURCE_KIND} ${RESOURCE_NAME} > ${NAMESPACE}/${RESOURCE_KIND}/${RESOURCE_NAME}.yaml"
    kubectl -n ${NAMESPACE} get ${RESOURCE_KIND} ${RESOURCE_NAME} -o json | jq --sort-keys \
        'del(
            .metadata.annotations."deployment.kubernetes.io/revision",
            .metadata.annotations."ingress.cloud.tencent.com/direct-access",
            .metadata.annotations."kubectl.kubernetes.io/last-applied-configuration",
            .metadata.annotations."kubernetes.io/ingress.class",
            .metadata.annotations."kubernetes.io/ingress.existLbId",
            .metadata.annotations."kubernetes.io/ingress.extensiveParameters",
            .metadata.annotations."kubernetes.io/ingress.http-rules",
            .metadata.annotations."kubernetes.io/ingress.https-rules",
            .metadata.annotations."kubernetes.io/ingress.qcloud-loadbalance-id",
            .metadata.annotations."kubernetes.io/ingress.rule-mix",
            .metadata.annotations."nginx.ingress.kubernetes.io/server-alias",
            .metadata.annotations."qcloud_cert_id",
            .metadata.annotations."service.cloud.tencent.com/direct-access",
            .metadata.annotations."service.kubernetes.io/loadbalance-id",
            .metadata.annotations."service.kubernetes.io/qcloud-loadbalancer-clusterid",
            .metadata.annotations."service.kubernetes.io/qcloud-loadbalancer-internal-subnetid",
            .metadata.annotations."service.kubernetes.io/tke-existed-lbid",
            .metadata.creationTimestamp,
            .metadata.generation,
            .metadata.resourceVersion,
            .metadata.selfLink,
            .metadata.uid,
            .spec.clusterIP,
            .status
        )' | python -c 'import sys, yaml, json; yaml.safe_dump(json.load(sys.stdin), sys.stdout, default_flow_style=False)' > "${NAMESPACE}/${RESOURCE_KIND}/${RESOURCE_NAME}.yaml"

    # 将serivce 设置为 ClusterIP 类型，避免云平台场景 从平台申请 lb 地址 
    if [[ ${RESOURCE_KIND} = "service" ]]; then
        sed "/type:/c\type: ClusterIP" -i ${NAMESPACE}/${RESOURCE_KIND}/${RESOURCE_NAME}.yaml
        sed "/nodePort/d" -i ${NAMESPACE}/${RESOURCE_KIND}/${RESOURCE_NAME}.yaml
    elif [[ ${RESOURCE_KIND} = "ingress" ]]; then
        # 设置ingress类型为 nginx
        sed "/annotations/a\    kubernetes.io/ingress.class: nginx" -i ${NAMESPACE}/${RESOURCE_KIND}/${RESOURCE_NAME}.yaml
    fi

done
