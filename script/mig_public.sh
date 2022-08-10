#!/bin/bash
############ source mongodb connection info
SRC_USER=
SRC_PASS=
SRC_AUTH_DB=
SRC_HOST=
SRC_PORT=
############ destination mongodb connection info
DST_USER=
DST_PASS=
DST_AUTH_DB=
DST_HOST=
DST_PORT=

########### mongodb bin file path
MONGO_CLIENT=/home/worker/mongodb/bin/mongo
MONGO_DUMP=/home/worker/mongodb/bin/mongodump
BOSN_DUMP=/home/worker/mongodb/bin/bsondump
MONGO_RESTORE=/home/worker/mongodb/bin/mongorestore
MAX_COLLECTION_DUMP=16
TMP_FILE=/data1/public/
START_TIME=`date  +%Y%m%d%H%M%S`
SNAPSHOOT_DUMP_DIR_NAME=${TMP_FILE}/snapshoot_dump_${START_TIME}
OPLOG_JSON_FILENAME=oplog_${START_TIME}.json
DUMP_OPLOG_DIR_NAME=${TMP_FILE}/oplog_dump/${START_TIME}
LAST_OPTIME_FILE_NAME=${TMP_FILE}/last_optime.ts
LOCK_FILE=${TMP_FILE}/process.lock
########## snapshoot data sync
function snapshoot_sync(){
  $MONGO_DUMP -u $SRC_USER -p $SRC_PASS -h $SRC_HOST  -j $MAX_COLLECTION_DUMP -h  $SRC_HOST --port $SRC_PORT  --authenticationDatabase $SRC_AUTH_DB --oplog -o $SNAPSHOOT_DUMP_DIR_NAME
  if [ $? -gt 0 ] ; then
     echo "dump snapshoot data error"
     rm -f $LOCK_FILE
     exit 99
  fi
  rm -rf $SNAPSHOOT_DUMP_DIR_NAME/admin # 删除admin相关数据文件，否则报错
  ## 不还原oplog,使用脚本统一dump oplog进行还原
  $MONGO_RESTORE  -u $DST_USER -p $DST_PASS -h $DST_HOST --port $DST_PORT --authenticationDatabase $DST_AUTH_DB  --verbose=4  -j $MAX_COLLECTION_DUMP --drop  $SNAPSHOOT_DUMP_DIR_NAME
  if [ $? -gt 0 ] ; then
     echo "mongorestore snapshoot data error"
     rm -f $LOCK_FILE
     exit 100
  fi
  $BOSN_DUMP  --outFile=${TMP_FILE}/snapshoot_oplog.json  $SNAPSHOOT_DUMP_DIR_NAME/oplog.bson
  if [ $? -gt 0 ] ; then
     echo "conversion bson file  error"
     rm -f $LOCK_FILE
     exit 101
  fi
  echo "successful finish snapshoot data sync"
  snapshoot_ts=`tail -1 ${TMP_FILE}/snapshoot_oplog.json| awk -F ',' '{print$1}'|awk -F ':' '{print $4}'`
  snapshoot_i=`tail -1 ${TMP_FILE}/snapshoot_oplog.json| awk -F ',' '{print$2}'|awk -F '}}' '{print $1}' |awk -F ':' '{print $2}'`
  echo $snapshoot_ts,$snapshoot_i>$LAST_OPTIME_FILE_NAME # 写入第一次快照同步的oplog时间
  echo "write start oplog $snapshoot_ts,$snapshoot_i"
}
######### increment data sync
function increment_sync(){
last_optime=`cat $LAST_OPTIME_FILE_NAME|egrep '^[0-9]{10}\,[0-9]{1,6}$'`
if [ ${#last_optime} -gt 11 ] ; then
  echo "found last oplog timestamp $last_optime"
  echo "start dump oplog start optime $last_optime"
  ts=`echo $last_optime |awk -F ',' '{print $1}'`
  i=`echo $last_optime |awk -F ',' '{print $2}'`
  echo "check oplog exist"
  exists=`$MONGO_CLIENT -u $SRC_USER -p $SRC_PASS --host $SRC_HOST --port $SRC_PORT --authenticationDatabase $SRC_AUTH_DB --quiet --eval 'db.oplog.rs.find({ts:{$eq: Timestamp('$ts','$i')}}).count()' local`
  if [ $exists -ne '1' ] ; then
    echo "oplog timestamp did not found,may be oplog has been overwritten!"
  exit 999
  else
    echo "oplog timestamp ok,start dump oplog[Timestamp($ts,$i)]"
  fi
  $MONGO_DUMP -u $SRC_USER -p $SRC_PASS -h $SRC_HOST --port $SRC_PORT --authenticationDatabase $SRC_AUTH_DB -d local -c oplog.rs --query '{ts:{$gt: Timestamp('$ts','$i')},ns:{$regex:"^(?!admin).*"}}' -o $DUMP_OPLOG_DIR_NAME
  if [ $? -gt 0 ] ; then
     echo 'dump oplog error'
     rm -f $LOCK_FILE
     exit 9
  fi
  $BOSN_DUMP  --outFile=${DUMP_OPLOG_DIR_NAME}/${OPLOG_JSON_FILENAME}  ${DUMP_OPLOG_DIR_NAME}/local/oplog.rs.bson
  if [ $? -gt 0 ] ; then
     echo 'convert oplog bson file(${DUMP_OPLOG_DIR_NAME}/local/oplog.rs.bson) error'
     rm -f $LOCK_FILE
     exit 9
  fi

  records=`cat ${DUMP_OPLOG_DIR_NAME}/$OPLOG_JSON_FILENAME |wc -l`
  if [ $records -ne '0' ] ; then
    end_ts=`tail -1 ${DUMP_OPLOG_DIR_NAME}/$OPLOG_JSON_FILENAME | awk -F ',' '{print$1}'|awk -F ':' '{print $4}'`
    end_i=`tail -1 ${DUMP_OPLOG_DIR_NAME}/$OPLOG_JSON_FILENAME | awk -F ',' '{print$2}'|awk -F '}}' '{print $1}' |awk -F ':' '{print $2}'`
    echo successful dump oplog record $records
    echo start restore oplog...
    $MONGO_RESTORE -u $DST_USER -p $DST_PASS -h $DST_HOST --port $DST_PORT --authenticationDatabase $DST_AUTH_DB --oplogReplay ${DUMP_OPLOG_DIR_NAME}/local/oplog.rs.bson
    if [ $? -gt 0 ] ; then
      echo "restore oplog error, please check data!"
      exit 99
    fi
    echo $end_ts,$end_i>$LAST_OPTIME_FILE_NAME # 修改最后一次导出的oplog time
    echo "successful restore oplog file ${DUMP_OPLOG_DIR_NAME}/local/oplog.rs.bson"
  else
    echo "oplog records not found ,exit..."
  fi
else
  echo "last oplog format invaild exit!"
  exit 9
fi
}
############## command check
if [ ! -f $MONGO_CLIENT ] ; then
  echo "mongo command not found :$MONGO_CLIENT,exit!"
  exit 2
fi
if [ ! -f $MONGO_DUMP ] ; then
  echo "mongodump command not found :$MONGO_DUMP,exit!"
  exit 2
fi
if [ ! -f $BOSN_DUMP ] ; then
  echo "bsondump command not found :$BOSN_DUMP,exit!"
  exit 2
fi
if [ ! -f $MONGO_RESTORE ] ; then
  echo "mongorestore command not found :$MONGO_RESTORE,exit!"
  exit 2
fi
############### dir check
if [ ! -d "${TMP_FILE}/oplog_dump" ];then
	  mkdir -p /data1/antifbackup/oplog_dump
fi
############### master secondary check
src_res=`$MONGO_CLIENT -u $SRC_USER -p $SRC_PASS --host $SRC_HOST --port $SRC_PORT --quiet  --authenticationDatabase $SRC_AUTH_DB  --eval 'db.adminCommand({isMaster:1})'|grep '"secondary" : true,'|wc -l`
if [ $src_res -ne '1' ] ; then
  echo $src_res
  echo "source mongodb node is not sencondary,exit!"
  exit 3
fi
dst_res=`$MONGO_CLIENT -u $DST_USER -p $DST_PASS --host $DST_HOST --port $DST_PORT --quiet  --authenticationDatabase $DST_AUTH_DB  --eval 'db.adminCommand({isMaster:1})'|grep '"ismaster" : true,'|wc -l`
if [ $dst_res -ne '1' ] ; then
  echo "destination mongodb node is not primary,exit!"
  exit 3
fi

############## run process
if [ -f $LOCK_FILE ] ; then
  echo "found process lock file $LOCK_FILE, may be another process is running  exit!" #进程锁文件，如果文件存在表示有进程在运行或在上一次进行备份还原时候出错导致锁文件未删除
  exit 4
fi
touch $LOCK_FILE
if [ ! -f $LAST_OPTIME_FILE_NAME ] ; then
  echo "oplog timestamp file not found,start snapshoot sync"
  snapshoot_sync
  echo "snapshoot sync successful,start increment sync"
  increment_sync
else
  echo "start increment sync"
  increment_sync
fi
rm -f $LOCK_FILE
