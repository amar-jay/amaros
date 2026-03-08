package topic_test

import (
	"testing"

	"github.com/amar-jay/amaros/pkg/topic"
)

func TestValidate(t *testing.T) {
	valid := []string{
		"/sensor.imu",
		"/robot.sensor.imu",
		"/robot.arm.joint1",
		"/cmd_vel",
		"/_private",
		"/sensor.imu_data",
	}
	for _, name := range valid {
		if err := topic.Validate(name); err != nil {
			t.Errorf("Validate(%q) unexpected error: %v", name, err)
		}
	}

	invalid := []string{
		"",
		"imu",
		"//imu",
		"/",
		".",
		"//imu..",
		"/sensor..imu",
		"//sensor.imu",
		"/1sensor",
		"/sensor/",
		"/sensor/imu",
		"/sensor imu",
	}
	for _, name := range invalid {
		if err := topic.Validate(name); err == nil {
			t.Errorf("Validate(%q) expected error, got nil", name)
		}
	}
}
