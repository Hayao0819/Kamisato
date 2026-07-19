package buildclient

import "context"

func RequestDeviceCode(ctx context.Context, base string) (DeviceCodeResponse, error) {
	c, err := ayato(base, "")
	if err != nil {
		return DeviceCodeResponse{}, err
	}
	return c.RequestDeviceCode(ctx)
}

func PollDeviceToken(ctx context.Context, base, deviceCode string) (DeviceTokenResult, error) {
	c, err := ayato(base, "")
	if err != nil {
		return DeviceTokenResult{}, err
	}
	return c.PollDeviceToken(ctx, deviceCode)
}
