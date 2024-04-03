package types

import (
	"encoding/json"
)

type MCEvent struct {
	Timestamp int64
	Type      MCEventType
	DataJson  string
	Data      MCEventData
}

type MCEventType int32

type MCEventData map[string]interface{}

const (
	WanConsistencyCheckStarted MCEventType = iota + 1
	WanConsistencyCheckFinished
	WanSyncStarted
	WanSyncFinishedFull
	WanConsistencyCheckIgnored
	WanSyncProgressUpdate
	WanSyncFinishedMerkle
	WanConfigurationAdded
	AddWanConfigurationIgnored
	WanSyncIgnored
	WanConfigurationExtended
)

const (
	uuid               = "uuid"
	mapName            = "mapName"
	wanReplicationName = "wanReplicationName"
	wanPublisherId     = "wanPublisherId"
	wanConfigName      = "wanConfigName"
	reason             = "reason"
	partitionsSynced   = "partitionsSynced"
	partitionsToSync   = "partitionsToSync"
	entriesToSync      = "entriesToSync"
	durationSecs       = "durationSecs"
	diffCount          = "diffCount"
	checkedCount       = "checkedCount"
)

func NewMCEvent(timestamp int64, eventType MCEventType, dataJson string) MCEvent {
	event := MCEvent{
		Timestamp: timestamp,
		Type:      eventType,
		DataJson:  dataJson,
	}
	eventData := make(map[string]interface{})
	if err := json.Unmarshal([]byte(dataJson), &eventData); err == nil {
		event.Data = eventData
	}
	return event
}

func (d MCEvent) WithReason(r string) MCEvent {
	return d.setProperty(reason, r)
}

func (d MCEvent) WithMapName(m string) MCEvent {
	return d.setProperty(mapName, m)
}

func (d MCEvent) Reason() string {
	return d.stringProperty(reason)
}

func (d MCEvent) MapName() string {
	return d.stringProperty(mapName)
}

func (d MCEvent) UUID() string {
	return d.stringProperty(uuid)
}

func (d MCEvent) PublisherId() string {
	return d.stringProperty(wanPublisherId)
}

func (d MCEvent) stringProperty(property string) string {
	if d.Data == nil {
		return ""
	}
	p, ok := d.Data[property].(string)
	if !ok {
		return ""
	}
	return p
}

func (d MCEvent) setProperty(property, value string) MCEvent {
	if d.Data == nil {
		d.Data = make(map[string]interface{})
	}
	d.Data[property] = value
	return d
}

func (e MCEventType) IsWanSync() bool {
	return e == WanSyncIgnored ||
		e == WanSyncFinishedFull ||
		e == WanSyncStarted ||
		e == WanSyncFinishedMerkle ||
		e == WanSyncProgressUpdate
}

func (e MCEventType) IsInProgress() bool {
	return e == WanConsistencyCheckStarted ||
		e == WanSyncStarted ||
		e == WanSyncProgressUpdate
}

func (e MCEventType) IsDone() bool {
	return e == WanConsistencyCheckFinished ||
		e == WanSyncFinishedFull ||
		e == WanSyncFinishedMerkle ||
		e == WanConfigurationAdded ||
		e == WanConfigurationExtended
}

func (e MCEventType) IsError() bool {
	return e == WanSyncIgnored ||
		e == AddWanConfigurationIgnored ||
		e == WanConsistencyCheckIgnored
}
