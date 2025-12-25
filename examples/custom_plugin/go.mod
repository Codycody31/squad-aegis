module example_custom_plugin

go 1.21

// Replace with the actual path to squad-aegis
// When building, ensure squad-aegis is in your GOPATH or use replace directive
require go.codycody31.dev/squad-aegis v0.0.0

replace go.codycody31.dev/squad-aegis => ../../

