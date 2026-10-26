package service

import "testing"

func TestValidateRechargeConfig(t *testing.T) {
	tests := []struct {
		name    string
		values  map[string]interface{}
		wantErr bool
	}{
		{
			name: "valid catfk shop",
			values: map[string]interface{}{
				"payment_compute_rate":  1000.0,
				"recharge_shop_enabled": true,
				"recharge_shop_url":     "https://catfk.com/shop/yardai",
			},
		},
		{
			name: "disabled shop may be empty",
			values: map[string]interface{}{
				"payment_compute_rate":  1000.0,
				"recharge_shop_enabled": false,
				"recharge_shop_url":     "",
			},
		},
		{
			name: "reject non-positive rate",
			values: map[string]interface{}{
				"payment_compute_rate":  0.0,
				"recharge_shop_enabled": false,
			},
			wantErr: true,
		},
		{
			name: "reject lookalike domain",
			values: map[string]interface{}{
				"payment_compute_rate":  1000.0,
				"recharge_shop_enabled": true,
				"recharge_shop_url":     "https://catfk.com.example.com/shop/yardai",
			},
			wantErr: true,
		},
		{
			name: "reject insecure shop URL",
			values: map[string]interface{}{
				"payment_compute_rate":  1000.0,
				"recharge_shop_enabled": true,
				"recharge_shop_url":     "http://catfk.com/shop/yardai",
			},
			wantErr: true,
		},
		{
			name: "reject non-standard port",
			values: map[string]interface{}{
				"payment_compute_rate":  1000.0,
				"recharge_shop_enabled": true,
				"recharge_shop_url":     "https://catfk.com:8443/shop/yardai",
			},
			wantErr: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateRechargeConfig(test.values)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateRechargeConfig() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
