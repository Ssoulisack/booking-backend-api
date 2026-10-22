package utils

import (
	"time"
)

func SafeString(ptr *string) string {
	if ptr != nil {
		return *ptr
	}
	return ""
}

func SafeInt(ptr *int) int {
	if ptr != nil {
		return *ptr
	}
	return 0
}

func SafeTime(ptr *time.Time) time.Time {
	if ptr != nil {
		return *ptr
	}
	return time.Time{}
}

func SafeFloat32(ptr *float32) float32 {
	if ptr != nil {
		return *ptr
	}
	return 0.0
}

func SafeFloat64(ptr *float64) float64 {
	if ptr != nil {
		return *ptr
	}
	return 0.0
}

func SafeBool(ptr *bool) bool {
	if ptr != nil {
		return *ptr
	}
	return false
}

func SafeUint(ptr *uint) uint {
	if ptr != nil {
		return *ptr
	}
	return 0
}

// for interface
func SafeInterfaceSlice(ptr *[]interface{}) []interface{} {
	if ptr != nil {
		return *ptr
	}
	return []interface{}{}
}

func SafeInterface(ptr *interface{}) interface{} {
	if ptr != nil {
		return *ptr
	}
	return nil
}

// for slice
func SafeStringSlice(ptr *[]string) []string {
	if ptr != nil {
		return *ptr
	}
	return []string{}
}

func SafeIntSlice(ptr *[]int) []int {
	if ptr != nil {
		return *ptr
	}
	return []int{}
}

func SafeFloat32Slice(ptr *[]float32) []float32 {
	if ptr != nil {
		return *ptr
	}
	return []float32{}
}

func SafeFloat64Slice(ptr *[]float64) []float64 {
	if ptr != nil {
		return *ptr
	}
	return []float64{}
}

func SafeBoolSlice(ptr *[]bool) []bool {
	if ptr != nil {
		return *ptr
	}
	return []bool{}
}

func SafeUintSlice(ptr *[]uint) []uint {
	if ptr != nil {
		return *ptr
	}
	return []uint{}
}

func SafeTimeSlice(ptr *[]time.Time) []time.Time {
	if ptr != nil {
		return *ptr
	}
	return []time.Time{}
}
