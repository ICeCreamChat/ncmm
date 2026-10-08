package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/3899/ncmm/api"
	"github.com/3899/ncmm/api/eapi"
	"github.com/3899/ncmm/pkg/cookie"
	"github.com/3899/ncmm/pkg/log"
)

// Query the task itself without the playback client's progress conversion.
// Only task fields are printed; cookies and request headers stay private.
type task struct {
	Name             string `json:"name"`
	MissionCode      string `json:"missionCode"`
	MissionStatus    int    `json:"missionStatus"`
	ProgressRate     int    `json:"progressRate"`
	TotalCompleteNum int    `json:"totalCompleteNum"`
	TaskProgressText string `json:"taskProgressText"`
	Desc             string `json:"desc"`
}

func newStatusClient(cookiePath string) (*api.Client, error) {
	loggerConfig := log.Config{Level: "error", Format: "text"}
	loggerConfig.Rotate.Filename = os.DevNull
	log.Default = log.New(&loggerConfig)
	return api.NewClient(&api.Config{
		Timeout: 30 * time.Second,
		Retry:   2,
		Cookie:  cookie.Config{Filepath: cookiePath},
	}, log.Default)
}

func queryStatus(ctx context.Context, client *api.Client) (*eapi.MusicianVipTasksResp, error) {
	return eapi.New(client).MusicianVipTasks(ctx, &eapi.MusicianVipTasksReq{ER: false})
}

func run(cookiePath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	client, err := newStatusClient(cookiePath)
	if err != nil {
		return err
	}
	defer client.Close(context.Background())
	response, err := queryStatus(ctx, client)
	if err != nil {
		return err
	}
	if response.Code != 200 {
		return fmt.Errorf("网易云任务查询失败，code=%d（请检查登录态）", response.Code)
	}
	if !response.Data.IsMusician || response.Data.FurtherTask == nil {
		return fmt.Errorf("当前账号没有可读取的音乐人进阶任务")
	}
	snapshot := struct {
		RecentPlayCount30 int    `json:"recentPlayCount30"`
		TaskStartTime     int64  `json:"taskStartTime"`
		Tasks             []task `json:"tasks"`
	}{RecentPlayCount30: response.Data.RecentPlayCount30, TaskStartTime: response.Data.FurtherTaskStartTime}
	fmt.Printf("近30天普通播放：%d 次（与任务进度分别记录）\n", snapshot.RecentPlayCount30)
	for _, child := range response.Data.FurtherTask.Children {
		snapshot.Tasks = append(snapshot.Tasks, task{
			Name: child.Name, MissionCode: child.MissionCode, MissionStatus: child.MissionStatus,
			ProgressRate: child.ProgressRate, TotalCompleteNum: child.TotalCompleteNum,
			TaskProgressText: child.TaskProgressText, Desc: child.Desc,
		})
		fmt.Printf("    - 任务: %s — 状态: %d, 进度: %d/%d\n", child.Name, child.MissionStatus, child.ProgressRate, child.TotalCompleteNum)
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	fmt.Printf("NETEASE_TASK_SNAPSHOT=%s\n", data)
	return nil
}

func main() {
	cookiePath := flag.String("cookie", "", "Main account cookie file")
	flag.Parse()
	if *cookiePath == "" {
		fmt.Fprintln(os.Stderr, "Missing -cookie")
		os.Exit(1)
	}
	if err := run(*cookiePath); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
