package constants

import (
	"backend/internal/config"
	"fmt"
)

const CUSTOM_LOG_FORMAT string = "REQUEST_AT[${time}] PID[${pid}] REQUESTID[${locals:requestid}] RESSTATUS[${status}] - LATENCY[${latency}] METHOD[${method}] PATH[${path}] REFERER[${referer}] PROTOCOL[${protocol}] PORT[${port}] IP[${ip}] IPS[${ips}] HOST[${host}] UA[${ua}] REQHEADERS[${reqHeaders}] REQQUERYPARAMS[${queryParams}] \n URL[${url}]\n REQBODY[${body}] REQHEADER:[${header:}] REQHEADER:[${reqHeader:}] REQQUERY[${query:}] REQFORM[${form:}] REQCOOKIE[${cookie:}] \n RESBODY[${resBody}]\n BYTESSENT[${bytesSent}] BYTESRECEIVED[${bytesReceived}] ROUTE[${route}] ERROR[${error}]  RESPHEADER:[${respHeader:}]  LOCALS:[${locals:}]\n <------------------------------------------------------------------------------------> \n"

var APP_DATE_TIME_LAYOUT_FORMAT = "2006-01-02 15:04:05.000000 -0700 -07 MST m=+0.000000000"
var APP_DATE_FORMAT = "2006-01-02"
var APP_DATEFORMAT = "20060102"
var APP_DATE_TIME_FORMAT = "2006-01-02 15:04:05"

const APP_TIME_ZONE = "Asia/Bangkok"

func GetMinioURL(key string) string {
	if key == "" {
		return ""
	}
	url := fmt.Sprintf("%s/%s/%s", config.MinioGlobal.BaseUrl, config.MinioGlobal.BucketName, key)
	return url
}
