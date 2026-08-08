package contracts

type returnScheduleBody struct {
	ReturnScheduleMode *string `json:"return_schedule_mode"`
	ReturnDelayValue   *int    `json:"return_delay_value"`
	ReturnDelayUnit    *string `json:"return_delay_unit"`
	ReturnDelaySeconds *int    `json:"return_delay_seconds"`
	ReturnTime         *string `json:"return_time"`
	ReturnWeekdays     []int   `json:"return_weekdays"`
}

func (b returnScheduleBody) patch() (ReturnSchedulePatch, bool) {
	has := b.ReturnScheduleMode != nil || b.ReturnDelayValue != nil || b.ReturnDelayUnit != nil ||
		b.ReturnDelaySeconds != nil || b.ReturnTime != nil || b.ReturnWeekdays != nil
	return ReturnSchedulePatch{
		Mode:              b.ReturnScheduleMode,
		DelayValue:        b.ReturnDelayValue,
		DelayUnit:         b.ReturnDelayUnit,
		DelaySeconds:      b.ReturnDelaySeconds,
		ReturnTime:        b.ReturnTime,
		ReturnWeekdays:    append([]int(nil), b.ReturnWeekdays...),
		ReturnWeekdaysSet: b.ReturnWeekdays != nil,
	}, has
}
