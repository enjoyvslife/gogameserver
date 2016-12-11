package gamedatamanager


import (
	"log"
	"reflect"
	"encoding/json"

	"gogameserver/util"
	dt  "gogameserver/datatypes" 
    rcl "gogameserver/redisclient" 
)

const REDIS_NIL string  = "redis: nil"

type GameManager struct {
	rc *rcl.RedisClient
}

func New() (gm * GameManager) {
    return &GameManager{
        rc: rcl.New(),
    }
}

func (gm * GameManager) GetPlayerData(gameName string, playerId string) (string, bool) {
	playerDataStr, err := gm.rc.GetVal(gameName+playerId)
	if err != nil && err.Error() != REDIS_NIL {
		go log.Printf("\nERROR:GetPlayerData: Game %s, playerId %s not found, err:%v", gameName, playerId, err)
		return "player not found", false
	} else {
		return playerDataStr, true
	}
}

func (gm * GameManager) DelPlayerData(gameName string, playerId string) (int64, bool) {
	redisRet, err := gm.rc.DelKey(gameName+playerId)
	if err != nil && err.Error() != REDIS_NIL {
		go log.Printf("\nERROR:DelPlayerData: Game %s, playerId %s not found, err:%v", gameName, playerId, err)
		return -1, false
	} else {
		return redisRet, true
	}
}

// store player data
func (gm * GameManager) StorePlayerData(gameName string, playerData dt.PlayerData) (bool){
	err := gm.rc.SaveKeyValForever(gameName+playerData.I, dt.Str(playerData))
	if err != nil && err.Error() != REDIS_NIL {
		go log.Printf("\nERROR:StorePlayerData: Game %s, playerData %v, err:%v", gameName, playerData, err)
		return false
	} else {
		go log.Printf("\nInfo: Success StorePlayerData: Game %s, playerData %v", gameName, playerData)
		return true
	}
}

// store player new score
func (gm * GameManager) StorePlayerScore(gameName string,  score float64, playerId string) (bool){
	currHiScore, err := gm.GetPlayerHighScore(gameName, playerId)
	if err != nil && err.Error() != REDIS_NIL {
		log.Printf("ERROR:StorePlayerScore: Game %s, playerId %s not found. err:'%s'", gameName, playerId, err.Error() )
		return false
	} else {
		if currHiScore < score {
			pDataStr, found := gm.GetPlayerData(gameName, playerId)
			if !found {
				return false
			}
			pData := dt.JsonFromStr(pDataStr)
			pData.A = score 
			// a go routine to update 
			gm.StorePlayerData(gameName, pData)
			gm.StorePlayerScoreDaily(gameName, score, playerId)
			redisRet, redisErr := gm.rc.AddToSet(gameName, score, playerId)
			if redisErr != nil && redisErr.Error() != REDIS_NIL {
				go log.Printf("Error:AddToSet: SUCESS gameName:%s, score:%f, playerId:%s, redisErr:%v", gameName, score, playerId, redisErr)
				return false
			}	
			go log.Printf("Info:StorePlayerScore: SUCESS currHiScore:%.6f, newScore:%.6f, Game %s, playerId %s, retcode:%d", currHiScore, score, gameName, playerId, redisRet)
		} else {
			go log.Printf("Info:StorePlayerScore: Already high currHiScore:%.6f, newScore:%.6f, Game %s, playerId %s", currHiScore, score, gameName, playerId)
		}
		return true
	}
}

// delete player score
func (gm * GameManager) DeletePlayerScore(gameName string,  playerId string) (bool){
	redisRet, redisErr := gm.rc.RemScore(gameName, playerId)
	if redisErr != nil && redisErr.Error() != REDIS_NIL {
		go log.Printf("Error:DeletePlayerScore: SUCESS gameName:%s, playerId:%s, redisErr:%v", gameName, playerId, redisErr)
		return false
	} else {
		go log.Printf("Info :DeletePlayerScore: SUCESS gameName:%s,  playerId:%s, redisRet:%d", gameName, playerId, redisRet)
	}
	return true
}

// get score
func (gm * GameManager) GetPlayerHighScore(gameName string, playerId string) (float64, error) {
	return gm.rc.GetScore(gameName, playerId)
}

// get player rank
func (gm * GameManager) GetPlayerRank(gameName string, playerId string) (int64) {
	rank, err := gm.rc.GetRank(gameName, playerId)
	if err != nil && err.Error() != REDIS_NIL {
		go log.Printf("Error:GetPlayerRank: Game %s, playerId %s", gameName, playerId)
		return -1;
	} else {
		return rank+1;
	}
}

// get top x
func (gm * GameManager) GetTopPlayers(gameName string, top int64) (string) {
    var topPlayersWithScores dt.ResponseData

	topResults, err := gm.rc.GetTop(gameName, top)
	if err != nil && err.Error() != REDIS_NIL{
		go log.Printf("Error:GetTopPlayers: Game %s", gameName)
		return ""
	}
    topResultsVal := reflect.ValueOf(topResults)
    log.Printf("Info: GetTopPlayers: topResultsVal %v", topResultsVal)
    resultCount := topResultsVal.Len()
    topPlayersWithScores.PlayerIds = make([]string, resultCount)
    topPlayersWithScores.Scores = make([]float64, resultCount)
    for i:=0; i<resultCount; i++ {
    	_score,_  := topResultsVal.Index(i).Field(0).Interface().(float64)
    	_pid,_  := topResultsVal.Index(i).Field(1).Interface().(string)
        topPlayersWithScores.PlayerIds[i] = _pid
        topPlayersWithScores.Scores[i] = _score
    }
    log.Printf("Info: GetTopPlayers: topPlayersWithScores %v", topPlayersWithScores)
    b, jerr := json.Marshal(topPlayersWithScores)
	if jerr == nil {
		return string(b)
	} else {
		return "json error"
	}
}

// store player new score for a week
func (gm * GameManager) StorePlayerScoreDaily(gameName string, score float64, playerId string) {
	gm.rc.AddToSet(gameName+util.CurrentDate(), score, playerId)
}

// get top weekly 1000
func (gm * GameManager) GetTopPlayersOnDay(gameName string, topCount int64, numOfDaysOld int) (string) {
	if numOfDaysOld > 6 {
	  return "";
	}
	return gm.GetTopPlayers(gameName+util.GetDate(numOfDaysOld), topCount)
}

// get rank among friends
func (gm * GameManager) GetScoreOfFriends(gameName string, playerId string, friendIds []string) (string) {
	var topPlayersWithScores dt.ResponseData
	playerScore, err := gm.GetPlayerHighScore(gameName, playerId)
	if err != nil && err.Error() != REDIS_NIL {
		go log.Printf("Error:GetRankAmongFriends: Error: %v", err)
		return ""
	}
    
	totalCount := len(friendIds) + 1
    topPlayersWithScores.PlayerIds = make([]string, totalCount)
    topPlayersWithScores.Scores = make([]float64, totalCount)
	for i:=1; i<totalCount; i++ {
		topPlayersWithScores.PlayerIds[i] = friendIds[i-1]
		topPlayersWithScores.Scores[i] = -1
		score, err1 := gm.GetPlayerHighScore(gameName, friendIds[i-1])
		if err1 != nil && err1.Error() != REDIS_NIL {
			go log.Printf("Error:GetTopPlayers: Game %s, %v", gameName, err1)
		} else {
			topPlayersWithScores.Scores[i] = score
		}
	}
	
	topPlayersWithScores.PlayerIds[0] = playerId
	topPlayersWithScores.Scores[0] =playerScore
	b, jerr := json.Marshal(topPlayersWithScores)
	if jerr == nil {
		return string(b)
	} else {
		return "json error"
	}
}


