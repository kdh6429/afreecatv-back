package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"strconv"
	"time"
)

type BJAPIData struct {
	Broad_no       string `json:"broad_no"`
	User_id        string `json:"user_id"`
	User_nick      string `json:"user_nick"`
	Broad_title    string `json:"broad_title"`
	Broad_thumb    string `json:"broad_thumb"`
	Total_view_cnt string `json:"total_view_cnt"`
}

type AfreecaAPIData struct {
	Total_cnt string `json:"total_cnt"`
	Time      int64  `json:"time"`
	//Cnt       int32      `json:"cnt"`
	Broad []BJAPIData `json:"broad"`
}

// n : ds
// d : [ { t: 202020, c: 392 }. {} ]
// nick, id, board_id, time, count, (optional)thubmnail

type TimeData struct {
	Nick        string
	User_id     string
	Board_id    string
	Up_ment     string
	Thumbnail   string
	Broad_title string
	Prev_count  int32
	Cur_count   int32
	Time        int64
}

type AFData struct {
	Type string
	Time int64
	Data []TimeData
}

type StreamInfo struct {
	sec_counts []int32
	min_counts []int32
	last_live  int64
	count      int32
}

var streamInfoByStreamId = map[string]StreamInfo{}

var prevData = AFData{}
var client = &http.Client{Timeout: 5 * time.Second}
var min_check_time = int64(0)
var preTime = int64(0)
var culLiveUp = AFData{}

func getAPIData(page int) (string, error) {
	//https://static.file.afreecatv.com/pc/ko_KR/main_broad_list.js
	//req, err := http.NewRequest("GET", "https://google.com/", nil)
	req, err := http.NewRequest("GET", "https://live.afreecatv.com/api/main_broad_list_api.php?pageNo="+fmt.Sprint(page)+"&orderType=view_cnt&selectType=action", nil)
	if err != nil {
		//fmt.Println(err.Error())
		return "", err
		//panic(err)
	}
	req.Header.Add("User-Agent", "Mozilla/5.0")

	//client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println(err.Error())
		return "", err
		//panic(err)
	}
	defer resp.Body.Close()

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		//fmt.Println(err.Error())
		return "", err
		//panic(err)
	}

	return string(bytes), nil
}

func toBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
func getByteFromUrlImage(url string) string {
	response, err1 := http.Get(url)
	if err1 != nil {
		return "IMAGE CONVERT ERROR1"
	}
	defer response.Body.Close()

	bytes, err2 := ioutil.ReadAll(response.Body)
	if err2 != nil {
		return "IMAGE CONVERT ERROR2"
	}
	var base64Encoding string

	mimeType := http.DetectContentType(bytes)
	switch mimeType {
	case "image/jpeg":
		base64Encoding += "data:image/jpeg;base64,"
	case "image/png":
		base64Encoding += "data:image/png;base64,"
	}
	base64Encoding += toBase64(bytes)

	return base64Encoding
}

func GetPrevData() (AFData, AFData) {
	return prevData, culLiveUp
}
func GetLiveData() (AFData, AFData, error) {
	afreeca_data := AfreecaAPIData{}

	for page := 1; page <= 3; page++ {
		raw_data, err := getAPIData(page)
		if err != nil {
			fmt.Println(err.Error())
			return AFData{}, AFData{}, err
		}
		_ = raw_data
		var tmp_data AfreecaAPIData
		err = json.Unmarshal([]byte(raw_data), &tmp_data)
		if err != nil {
			//fmt.Println(err.Error())
			return AFData{}, AFData{}, err
			//invalid character '\'' looking for beginning of object key string
		}

		afreeca_data.Broad = append(afreeca_data.Broad, tmp_data.Broad...)
		afreeca_data.Time = tmp_data.Time
		afreeca_data.Total_cnt = tmp_data.Total_cnt
	}
	curTime := afreeca_data.Time
	broads := len(afreeca_data.Broad)

	var liveData = AFData{}
	liveData.Type = "live"
	liveData.Data = []TimeData{}

	if preTime == curTime {
		//log.Printf("No update")
		return AFData{}, AFData{}, nil
	}
	//log.Printf("prevData length from start : %d", len(prevData.Data))

	//lastCountByBJ
	tmpTimeDatas := []TimeData{}
	tmpUpTimeDatas := []TimeData{}
	for i := 0; i < broads; i++ {
		count := func() int32 {
			tmp_count, err := strconv.ParseUint(afreeca_data.Broad[i].Total_view_cnt, 10, 32)
			if err != nil {
				tmp_count = 0
			}
			return int32(tmp_count)
		}()
		nick := afreeca_data.Broad[i].User_nick
		user_id := afreeca_data.Broad[i].User_id
		board_id := afreeca_data.Broad[i].Broad_no
		stream_id := user_id + "_" + board_id
		broad_title := afreeca_data.Broad[i].Broad_title
		thumbnail_url := afreeca_data.Broad[i].Broad_thumb
		thumbnail := ""
		up_ment := ""
		_ = broad_title
		_ = thumbnail
		// if stream is just started
		if _, ok := streamInfoByStreamId[stream_id]; !ok {
			//log.Printf(nick + " stream(" + stream_id + ")has been opened.")
			streamInfoByStreamId[stream_id] = StreamInfo{
				sec_counts: []int32{},
				min_counts: []int32{},
				last_live:  0,
			}
		}

		if streamInfoByStreamId[stream_id].last_live == curTime {
			continue
		}

		// get last stream count
		lastStreamInfo := streamInfoByStreamId[stream_id]
		lastStreamInfo.last_live = curTime

		// check upup - sec type
		sec_count_len := len(lastStreamInfo.sec_counts)

		// fmt.Printf("last counts : %v\n", lastStreamInfo.sec_counts)
		// fmt.Printf("cur counts : %d\n", count)
		if up_ment == "" && sec_count_len >= 1 {
			last_count := lastStreamInfo.sec_counts[sec_count_len-1]
			up_percent := float32(count) / float32(last_count)
			up_count := count - last_count
			threshold := int32(math.Sqrt(float64(last_count)) * 0.7)
			if threshold < up_count {
				up_ment = fmt.Sprintf("10초 만에 시청자 %d명(%.0f%%)↑", up_count, (up_percent-1)*100)
				lastStreamInfo.sec_counts = []int32{}
			}
		}
		if up_ment == "" && sec_count_len >= 3 {
			last_count := lastStreamInfo.sec_counts[sec_count_len-3]
			up_percent := float32(count) / float32(last_count)
			up_count := count - last_count
			threshold := int32(math.Sqrt(float64(last_count)))
			if threshold < up_count {
				up_ment = fmt.Sprintf("30초 만에 시청자 %d명(%.0f%%)↑", up_count, (up_percent-1)*100)
				lastStreamInfo.sec_counts = []int32{count}
			}
		}

		// sec check update
		lastStreamInfo.sec_counts = append(lastStreamInfo.sec_counts, count)
		if len(lastStreamInfo.sec_counts) > 3 {
			lastStreamInfo.sec_counts = lastStreamInfo.sec_counts[len(lastStreamInfo.sec_counts)-3:]
		}

		if min_check_time < curTime-60 {
			min_check_time = curTime

			// check upup - min type
			min_count_len := len(lastStreamInfo.min_counts)
			if up_ment == "" && min_count_len >= 1 {
				last_count := lastStreamInfo.min_counts[min_count_len-1]
				up_percent := float32(count) / float32(last_count)
				up_count := count - last_count
				threshold := int32(math.Sqrt(float64(last_count)) * 1.5)
				if threshold < up_count {
					up_ment = fmt.Sprintf("1분 만에 시청자 %d명(%.0f%%)↑", up_count, (up_percent-1)*100)
				}
			}
			if up_ment == "" && min_count_len >= 2 {
				last_count := lastStreamInfo.min_counts[min_count_len-2]
				up_percent := float32(count) / float32(last_count)
				up_count := count - last_count
				threshold := int32(math.Sqrt(float64(last_count)) * 2)
				if threshold < up_count {
					up_ment = fmt.Sprintf("2분 만에 시청자 %d명(%.0f%%)↑", up_count, (up_percent-1)*100)
				}
			}
			if up_ment == "" && min_count_len >= 3 {
				last_count := lastStreamInfo.min_counts[min_count_len-3]
				up_percent := float32(count) / float32(last_count)
				up_count := count - last_count
				threshold := int32(math.Sqrt(float64(last_count)) * 3)
				if threshold < up_count {
					up_ment = fmt.Sprintf("3분 만에 시청자 %d명(%.0f%%)↑", up_count, (up_percent-1)*100)
				}
			}

			// min check update
			lastStreamInfo.min_counts = append(lastStreamInfo.min_counts, count)
			if len(lastStreamInfo.min_counts) > 3 {
				lastStreamInfo.min_counts = lastStreamInfo.min_counts[len(lastStreamInfo.min_counts)-3:]
				//log.Printf("lastStreamInfo.min_counts delete : %d", len(lastStreamInfo.min_counts))
			}
		}

		if up_ment != "" {
			thumbnail = getByteFromUrlImage("http:" + thumbnail_url)
			tmpUpTimeDatas = append(tmpUpTimeDatas, TimeData{
				Nick:        nick,
				User_id:     user_id,
				Board_id:    board_id,
				Time:        curTime,
				Prev_count:  lastStreamInfo.count,
				Cur_count:   count,
				Up_ment:     up_ment,
				Thumbnail:   thumbnail,
				Broad_title: broad_title,
			})
		}

		tmpTimeDatas = append(tmpTimeDatas, TimeData{
			Nick:        nick,
			User_id:     user_id,
			Board_id:    board_id,
			Time:        curTime,
			Prev_count:  lastStreamInfo.count,
			Cur_count:   count,
			Up_ment:     up_ment,
			Thumbnail:   thumbnail,
			Broad_title: broad_title,
		})

		lastStreamInfo.count = count
		streamInfoByStreamId[stream_id] = lastStreamInfo
	}

	for stream_id := range streamInfoByStreamId {
		curStream := streamInfoByStreamId[stream_id]
		if curStream.last_live < curTime {
			delete(streamInfoByStreamId, stream_id)
		}
	}

	liveUp := AFData{}
	liveUp.Type = "up"
	liveUp.Data = tmpUpTimeDatas

	culLiveUp.Type = "prevup"
	culLiveUp.Data = append(culLiveUp.Data, tmpUpTimeDatas...)
	if len(culLiveUp.Data) > 20 {
		culLiveUp.Data = culLiveUp.Data[len(culLiveUp.Data)-20:]
	}

	// clear liveData object
	liveData.Data = tmpTimeDatas
	liveData.Time = curTime
	preTime = curTime

	prevData = liveData
	prevData.Type = "prev"
	return liveData, liveUp, nil
}
