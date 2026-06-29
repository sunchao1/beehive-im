package conf

import (
	"os"
	"path/filepath"
	"strings"

	"beehive-im/lib/log"
	"beehive-im/lib/rtmq"
)

/* 在线中心配置 */
type UsrSvrConf struct {
	Id       uint32           // 结点ID
	Gid      uint32           // 分组ID
	Port     int16            // HTTP侦听端口
	WorkPath string           // 工作路径(自动获取)
	AppPath  string           // 程序路径(自动获取)
	ConfPath string           // 配置路径(自动获取)
	Seqsvr   UsrSvrSeqsvrConf // SEQSVR配置
	Redis    UsrSvrRedisConf  // REDIS配置
	UserDb   UsrSvrMysqlConf  // USERDB配置(MYSQL)
	Mongo    UsrSvrMongoConf  // MONGO配置
	Cipher   string           // 私密密钥
	Log      log.Conf         // 日志配置
	Frwder   rtmq.ProxyConf   // RTMQ配置
	// IplistStatic：非空时 /im/iplist 直接返回固定列表（K8s Ingress 方案 A），跳过 Redis 字典。
	IplistStatic UsrSvrIplistStaticConf
}

/* 静态 iplist（可选） */
type UsrSvrIplistStaticConf struct {
	Tcp []string // type=1 TCP listend
	Ws  []string // type=2 WebSocket
}

/******************************************************************************
 **函数名称: Load
 **功    能: 加载配置信息
 **输入参数:
 **     path: 配置路径
 **输出参数: NONE
 **返    回:
 **     conf: 配置信息
 **     err: 错误描述
 **实现描述:
 **注意事项:
 **作    者: # Qifeng.zou # 2016.10.30 22:35:28 #
 ******************************************************************************/
func Load(path string) (conf *UsrSvrConf, err error) {
	conf = &UsrSvrConf{}

	conf.WorkPath, _ = os.Getwd()
	conf.WorkPath, _ = filepath.Abs(conf.WorkPath)
	conf.AppPath, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	conf.ConfPath = path

	err = conf.parse()
	if nil != err {
		return nil, err
	}
	conf.applyEnvOverrides()
	return conf, err
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// applyEnvOverrides K8s / 容器注入，优先于 Redis 侦听字典。
func (conf *UsrSvrConf) applyEnvOverrides() {
	if v := os.Getenv("BEEHIVE_WS_IPLIST"); v != "" {
		conf.IplistStatic.Ws = splitCSV(v)
	}
	if v := os.Getenv("BEEHIVE_TCP_IPLIST"); v != "" {
		conf.IplistStatic.Tcp = splitCSV(v)
	}
}

// StaticIplistForType 返回 type 对应的静态列表；nil 表示走 Redis 字典。
func (conf *UsrSvrConf) StaticIplistForType(typ int) []string {
	switch typ {
	case 1:
		if len(conf.IplistStatic.Tcp) > 0 {
			return conf.IplistStatic.Tcp
		}
	case 2:
		if len(conf.IplistStatic.Ws) > 0 {
			return conf.IplistStatic.Ws
		}
	}
	return nil
}

/* 获取结点ID */
func (conf *UsrSvrConf) GetNid() uint32 {
	return conf.Id
}

/* 获取分组ID */
func (conf *UsrSvrConf) GetGid() uint32 {
	return conf.Gid
}
