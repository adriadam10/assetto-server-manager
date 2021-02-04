package acserver

import "time"

var (
	TCPBytesRead        int
	TCPBytesWritten     int
	TCPMessagesReceived int
	TCPMessagesSent     int
	UDPBytesRead        int
	UDPBytesWritten     int
	UDPMessagesReceived int
	UDPMessagesSent     int
)

func clearStatistics() {
	TCPBytesRead = 0
	TCPBytesWritten = 0
	UDPBytesRead = 0
	UDPBytesWritten = 0
	TCPMessagesReceived = 0
	TCPMessagesSent = 0
	UDPMessagesReceived = 0
	UDPMessagesSent = 0
}

func printStatistics(logger Logger, currentTimeMilliseconds int64) {
	numSeconds := float64(currentTimeMilliseconds) / 1000

	logger.Infof("Statistics: Server was online for: %s", time.Duration(numSeconds)*time.Second)

	logger.Infof(
		"Statistics: UDP: %d bytes read, %d bytes written. TCP: %d bytes read, %d bytes written.",
		UDPBytesRead,
		UDPBytesWritten,
		TCPBytesRead,
		TCPBytesWritten,
	)

	logger.Infof(
		"Statistics: UDP: %d messages received, %d messages sent. TCP: %d messages received, %d messages sent.",
		UDPMessagesReceived,
		UDPMessagesSent,
		TCPMessagesReceived,
		TCPMessagesSent,
	)

	if UDPMessagesReceived == 0 || UDPMessagesSent == 0 || TCPMessagesReceived == 0 || TCPMessagesSent == 0 {
		return // avoid dividing by zero
	}

	logger.Infof(
		"Statistics: UDP: %d bytes avg receive size, %d bytes avg send size. TCP: %d bytes avg receive size, %d bytes avg send size",
		UDPBytesRead/UDPMessagesReceived,
		UDPBytesWritten/UDPMessagesSent,
		TCPBytesRead/TCPMessagesReceived,
		TCPBytesWritten/TCPMessagesSent,
	)

	logger.Infof(
		"Statistics: UDP: received %.2f messages/s, sent %.2f messages/s. TCP: received %.2f messages/s, sent %.2f messages/s",
		float64(UDPMessagesReceived)/numSeconds,
		float64(UDPMessagesSent)/numSeconds,
		float64(TCPMessagesReceived)/numSeconds,
		float64(TCPMessagesSent)/numSeconds,
	)
}
