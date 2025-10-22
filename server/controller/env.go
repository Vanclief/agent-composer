package controller

type EnvVars struct {
	Environment              string
	ProjectRootPath          string
	PromtailPassword         string ` mapstucture:"PROMTAIL_PASSWORD"`
	PostgresPassword         string ` mapstucture:"POSTGRES_PASSWORD"`
	AWSSecretKey             string ` mapstucture:"AWS_SECRET_KEY"`
	S3SecretKey              string ` mapstucture:"S3_SECRET_KEY"`
	STPSecretKey             string ` mapstucture:"STP_SECRET_KEY"`
	STPGatewayToken          string ` mapstucture:"STP_GATEWAY_TOKEN"`
	FacturamaPassword        string ` mapstucture:"FACTURAMA_PASSWORD"`
	HikDeviceGatewayPassword string ` mapstucture:"HIK_DEVICE_GATEWAY_PASSWORD"`
}
