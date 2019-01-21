
//此源码被清华学神尹成大魔王专业翻译分析并修改
//尹成QQ77025077
//尹成微信18510341407
//尹成所在QQ群721929980
//尹成邮箱 yinc13@mails.tsinghua.edu.cn
//尹成毕业于清华大学,微软区块链领域全球最有价值专家
//https://mvp.microsoft.com/zh-cn/PublicProfile/4033620
//版权所有2018 Go Ethereum作者
//此文件是Go以太坊库的一部分。
//
//Go-Ethereum库是免费软件：您可以重新分发它和/或修改
//根据GNU发布的较低通用公共许可证的条款
//自由软件基金会，或者许可证的第3版，或者
//（由您选择）任何更高版本。
//
//Go以太坊图书馆的发行目的是希望它会有用，
//但没有任何保证；甚至没有
//适销性或特定用途的适用性。见
//GNU较低的通用公共许可证，了解更多详细信息。
//
//你应该收到一份GNU较低级别的公共许可证副本
//以及Go以太坊图书馆。如果没有，请参见<http://www.gnu.org/licenses/>。

package dashboard

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/mohae/deepcopy"
	"github.com/rjeczalik/notify"
)

var emptyChunk = json.RawMessage("[]")

//preplogs从给定的日志记录缓冲区创建一个JSON数组。
//返回准备好的数组和最后一个'\n’的位置
//原始缓冲区中的字符，如果不包含任何字符，则为-1。
func prepLogs(buf []byte) (json.RawMessage, int) {
	b := make(json.RawMessage, 1, len(buf)+1)
	b[0] = '['
	b = append(b, buf...)
	last := -1
	for i := 1; i < len(b); i++ {
		if b[i] == '\n' {
			b[i] = ','
			last = i
		}
	}
	if last < 0 {
		return emptyChunk, -1
	}
	b[last] = ']'
	return b[:last+1], last - 1
}

//handleLogRequest searches for the log file specified by the timestamp of the
//请求，从中创建一个JSON数组并将其发送到请求客户机。
func (db *Dashboard) handleLogRequest(r *LogsRequest, c *client) {
	files, err := ioutil.ReadDir(db.logdir)
	if err != nil {
		log.Warn("Failed to open logdir", "path", db.logdir, "err", err)
		return
	}
	re := regexp.MustCompile(`\.log$`)
	fileNames := make([]string, 0, len(files))
	for _, f := range files {
		if f.Mode().IsRegular() && re.MatchString(f.Name()) {
			fileNames = append(fileNames, f.Name())
		}
	}
	if len(fileNames) < 1 {
		log.Warn("No log files in logdir", "path", db.logdir)
		return
	}
	idx := sort.Search(len(fileNames), func(idx int) bool {
//返回最小的索引，如文件名[idx]>=r.name，
//if there is no such index, returns n.
		return fileNames[idx] >= r.Name
	})

	switch {
	case idx < 0:
		return
	case idx == 0 && r.Past:
		return
	case idx >= len(fileNames):
		return
	case r.Past:
		idx--
	case idx == len(fileNames)-1 && fileNames[idx] == r.Name:
		return
	case idx == len(fileNames)-1 || (idx == len(fileNames)-2 && fileNames[idx] == r.Name):
//最后一个文件会不断更新，其块会被流式处理，
//因此，为了避免在客户机端复制日志记录，需要
//handled differently. Its actual content is always saved in the history.
		db.lock.Lock()
		if db.history.Logs != nil {
			c.msg <- &Message{
				Logs: db.history.Logs,
			}
		}
		db.lock.Unlock()
		return
	case fileNames[idx] == r.Name:
		idx++
	}

	path := filepath.Join(db.logdir, fileNames[idx])
	var buf []byte
	if buf, err = ioutil.ReadFile(path); err != nil {
		log.Warn("Failed to read file", "path", path, "err", err)
		return
	}
	chunk, end := prepLogs(buf)
	if end < 0 {
		log.Warn("The file doesn't contain valid logs", "path", path)
		return
	}
	c.msg <- &Message{
		Logs: &LogsMessage{
			Source: &LogFile{
				Name: fileNames[idx],
				Last: r.Past && idx == 0,
			},
			Chunk: chunk,
		},
	}
}

//streamlogs监视文件系统，并在记录器写入时
//新的日志记录到文件中，收集它们，然后
//从中取出JSON数组并将其发送到客户机。
func (db *Dashboard) streamLogs() {
	defer db.wg.Done()
	var (
		err  error
		errc chan error
	)
	defer func() {
		if errc == nil {
			errc = <-db.quit
		}
		errc <- err
	}()

	files, err := ioutil.ReadDir(db.logdir)
	if err != nil {
		log.Warn("Failed to open logdir", "path", db.logdir, "err", err)
		return
	}
	var (
opened *os.File //打开的活动日志文件的文件描述符。
buf    []byte   //包含最近写入的日志块，但尚未发送到客户端。
	)

//由于时间戳的存在，日志记录总是按字母顺序写入最后一个文件。
	re := regexp.MustCompile(`\.log$`)
	i := len(files) - 1
	for i >= 0 && (!files[i].Mode().IsRegular() || !re.MatchString(files[i].Name())) {
		i--
	}
	if i < 0 {
		log.Warn("No log files in logdir", "path", db.logdir)
		return
	}
	if opened, err = os.OpenFile(filepath.Join(db.logdir, files[i].Name()), os.O_RDONLY, 0600); err != nil {
		log.Warn("Failed to open file", "name", files[i].Name(), "err", err)
		return
	}
defer opened.Close() //关闭最后打开的文件。
	fi, err := opened.Stat()
	if err != nil {
		log.Warn("Problem with file", "name", opened.Name(), "err", err)
		return
	}
	db.lock.Lock()
	db.history.Logs = &LogsMessage{
		Source: &LogFile{
			Name: fi.Name(),
			Last: true,
		},
		Chunk: emptyChunk,
	}
	db.lock.Unlock()

	watcher := make(chan notify.EventInfo, 10)
	if err := notify.Watch(db.logdir, watcher, notify.Create); err != nil {
		log.Warn("Failed to create file system watcher", "err", err)
		return
	}
	defer notify.Stop(watcher)

	ticker := time.NewTicker(db.config.Refresh)
	defer ticker.Stop()

loop:
	for err == nil || errc == nil {
		select {
		case event := <-watcher:
//确保创建了新的日志文件。
			if !re.Match([]byte(event.Path())) {
				break
			}
			if opened == nil {
				log.Warn("The last log file is not opened")
				break loop
			}
//新日志文件的名称总是更大，
//因为它是使用实际日志记录的时间创建的。
			if opened.Name() >= event.Path() {
				break
			}
//读取以前打开的文件的其余部分。
			chunk, err := ioutil.ReadAll(opened)
			if err != nil {
				log.Warn("Failed to read file", "name", opened.Name(), "err", err)
				break loop
			}
			buf = append(buf, chunk...)
			opened.Close()

			if chunk, last := prepLogs(buf); last >= 0 {
//发送以前打开的文件的其余部分。
				db.sendToAll(&Message{
					Logs: &LogsMessage{
						Chunk: chunk,
					},
				})
			}
			if opened, err = os.OpenFile(event.Path(), os.O_RDONLY, 0644); err != nil {
				log.Warn("Failed to open file", "name", event.Path(), "err", err)
				break loop
			}
			buf = buf[:0]

//Change the last file in the history.
			fi, err := opened.Stat()
			if err != nil {
				log.Warn("Problem with file", "name", opened.Name(), "err", err)
				break loop
			}
			db.lock.Lock()
			db.history.Logs.Source.Name = fi.Name()
			db.history.Logs.Chunk = emptyChunk
			db.lock.Unlock()
case <-ticker.C: //向客户端发送日志更新。
			if opened == nil {
				log.Warn("The last log file is not opened")
				break loop
			}
//读取自上次读取以来创建的新日志。
			chunk, err := ioutil.ReadAll(opened)
			if err != nil {
				log.Warn("Failed to read file", "name", opened.Name(), "err", err)
				break loop
			}
			b := append(buf, chunk...)

			chunk, last := prepLogs(b)
			if last < 0 {
				break
			}
//Only keep the invalid part of the buffer, which can be valid after the next read.
			buf = b[last+1:]

			var l *LogsMessage
//更新历史记录。
			db.lock.Lock()
			if bytes.Equal(db.history.Logs.Chunk, emptyChunk) {
				db.history.Logs.Chunk = chunk
				l = deepcopy.Copy(db.history.Logs).(*LogsMessage)
			} else {
				b = make([]byte, len(db.history.Logs.Chunk)+len(chunk)-1)
				copy(b, db.history.Logs.Chunk)
				b[len(db.history.Logs.Chunk)-1] = ','
				copy(b[len(db.history.Logs.Chunk):], chunk[1:])
				db.history.Logs.Chunk = b
				l = &LogsMessage{Chunk: chunk}
			}
			db.lock.Unlock()

			db.sendToAll(&Message{Logs: l})
		case errc = <-db.quit:
			break loop
		}
	}
}
