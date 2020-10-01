package acserver

import (
	"fmt"
	"strconv"
	"strings"
)

type AdminCommandManager struct {
	state          *ServerState
	sessionManager *SessionManager
	weatherManager *WeatherManager
	logger         Logger
}

func NewAdminCommandManager(state *ServerState, sessionManager *SessionManager, weatherManager *WeatherManager, logger Logger) *AdminCommandManager {
	return &AdminCommandManager{
		state:          state,
		sessionManager: sessionManager,
		weatherManager: weatherManager,
		logger:         logger,
	}
}

func (a AdminCommandManager) GetEntrantFromCommandSplit(commandSplit []string, commandEntrant *Car) *Car {
	var entrantToReturn *Car
	carIDToReturn, err := strconv.Atoi(commandSplit[1])

	if err != nil {
		// try getting entrant by name
		name := strings.Join(commandSplit[1:], " ")
		entrantToReturn = a.state.GetCarByName(name)

		if entrantToReturn == nil {
			// some commands have a number at the end, try excluding it
			name := strings.Join(commandSplit[1:len(commandSplit)-1], " ")
			entrantToReturn = a.state.GetCarByName(name)

			if entrantToReturn == nil {
				_ = a.state.SendChat(ServerCarID, commandEntrant.CarID, fmt.Sprintf("Could not find entrant %s", name), false)
			}
		}
	} else {
		entrantToReturn, _ = a.state.GetCarByID(CarID(carIDToReturn))
	}

	if entrantToReturn == nil {
		// try getting entrant by guid
		entrantToReturn = a.state.GetCarByGUID(commandSplit[1], true)

		if entrantToReturn == nil {
			_ = a.state.SendChat(ServerCarID, commandEntrant.CarID, fmt.Sprintf("Could not find entrant %s", commandSplit[1]), false)
			return nil
		}
	}

	if !entrantToReturn.IsConnected() {
		_ = a.state.SendChat(ServerCarID, commandEntrant.CarID, fmt.Sprintf("Car %d is not connected to the server", entrantToReturn.CarID), false)
		return nil
	}

	return entrantToReturn
}

func (a *AdminCommandManager) Command(entrant *Car, command string) error {
	commandSplit := strings.Split(command, " ")

	if len(commandSplit) == 0 {
		return nil
	}

	commandType := strings.ToLower(commandSplit[0])

	switch commandType {
	case "/kick", "kick_id":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /kick command! Use /admin to get permission", false)
		}

		if len(commandSplit) >= 2 {
			entrantToKick := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToKick != nil {
				err := a.state.Kick(entrantToKick.CarID, KickReasonGeneric)

				if err != nil {
					return err
				}
			}
		} else {
			err := a.state.SendChat(ServerCarID, entrant.CarID, "Kick commands require the car ID, GUID or name to be kicked! (e.g. /kick 3)", false)

			if err != nil {
				return err
			}
		}
	case "/ban", "/ban_id":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /ban command! Use /admin to get permission", false)
		}

		if len(commandSplit) >= 2 {
			entrantToBan := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToBan != nil {
				err := a.state.Kick(entrantToBan.CarID, KickReasonGeneric)

				if err != nil {
					return err
				}

				err = a.state.AddToBlockList(entrantToBan.Driver.GUID)

				if err != nil {
					a.logger.WithError(err).Errorf("Couldn't add %s to the server blocklist.json", entrantToBan.Driver.GUID)
					return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Couldn't add %s to the server blocklist.json", entrantToBan.Driver.Name), false)
				}

				return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Successfully added %s to the server blocklist.json", entrantToBan.Driver.Name), false)
			}
		} else {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Ban commands require the car ID, GUID or name to be kicked! (e.g. /ban 3)", false)
		}
	case "/next_session":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /next_session command! Use /admin to get permission", false)
		}

		a.sessionManager.NextSession(true)
		a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s instructed the server to change to the next session", entrant.Driver.Name), false)
	case "/restart_session":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /restart_session command! Use /admin to get permission", false)
		}

		a.sessionManager.RestartSession()
		a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s instructed the server to restart the session", entrant.Driver.Name), false)
	case "/ballast":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /ballast command! Use /admin to get permission", false)
		}

		if len(commandSplit) >= 2 {
			entrantToApplyBallast := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToApplyBallast != nil {
				ballastString := commandSplit[len(commandSplit)-1]

				ballast, err := strconv.ParseFloat(ballastString, 32)

				if err != nil {
					return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Could not parse ballast %s as a number!", ballastString), false)
				}

				if ballast > 5000 {
					ballast = 5000
				}

				if ballast < 0 {
					ballast = 0
				}

				entrantToApplyBallast.Ballast = float32(ballast)

				a.state.BroadcastUpdateBoP(entrantToApplyBallast)

				a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has set %s's ballast to %.0fkg!", entrant.Driver.Name, entrantToApplyBallast.Driver.Name, ballast), false)
			}
		} else {
			return a.state.SendChat(ServerCarID, entrant.CarID, "ballast commands require the car ID, GUID or name and ballast amount! (e.g. /ballast 5 80)", false)
		}
	case "/restrictor":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /restrictor command! Use /admin to get permission", false)
		}

		if len(commandSplit) >= 2 {
			entrantToApplyRestrictor := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToApplyRestrictor != nil {
				restrictorString := commandSplit[len(commandSplit)-1]

				restrictor, err := strconv.ParseFloat(restrictorString, 32)

				if err != nil {
					return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Could not parse restrictor %s as a number!", restrictorString), false)
				}

				if restrictor > 400 {
					restrictor = 400
				}

				if restrictor < 0 {
					restrictor = 0
				}

				entrantToApplyRestrictor.Restrictor = float32(restrictor)

				a.state.BroadcastUpdateBoP(entrantToApplyRestrictor)

				a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has set %s's restrictor to %.0f%%!", entrant.Driver.Name, entrantToApplyRestrictor.Driver.Name, restrictor), false)
			}
		} else {
			return a.state.SendChat(ServerCarID, entrant.CarID, "restrictor commands require the car ID, GUID or name and restrictor amount! (e.g. /ballast 5 80)", false)
		}
	case "/next_weather":
		if !entrant.IsAdmin {
			return a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /next_weather command! Use /admin to get permission", false)
		}

		if a.weatherManager.weatherProgression {
			a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has changed the weather to the next configured weather!", entrant.Driver.Name), false)

			a.weatherManager.NextWeather()
		} else {
			return a.state.SendChat(ServerCarID, entrant.CarID, "This session does not have weather progression enabled! Look at the readme for more info+", false)
		}
	case "/help":
		if len(commandSplit) == 2 {
			if entrant.IsAdmin {
				switch strings.ToLower(commandSplit[1]) {
				case "kick":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Kick a driver from the server using car ID, GUID or name! (e.g. /kick 3)", false)
				case "ban":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Kick a driver from the server and add them to the block list using car ID, GUID or name! (e.g. /kick 3)", false)
				case "next_session":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Move to the next configured session, or back to the first session if loop mode is on", false)
				case "restart_session":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Restart the current session", false)
				case "client_list":
					return a.state.SendChat(ServerCarID, entrant.CarID, "See a list of clients in the current entry list", false)
				case "ballast":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Apply ballast (maximum 5000kg) to a driver from the server using car ID, GUID or name! (e.g. /ballast Kevin 40)", false)
				case "restrictor":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Apply an air intake restrictor (maximum 400%) to a driver from the server using car ID, GUID or name! (e.g. /restrictor Brad 40)", false)
				case "next_weather":
					return a.state.SendChat(ServerCarID, entrant.CarID, "Move to the next configured weather in the session", false)
				case "help":
					return a.state.SendChat(ServerCarID, entrant.CarID, "The help command provides context for server commands, just like this!", false)
				case "admin":
					return a.state.SendChat(ServerCarID, entrant.CarID, "The admin command will give you access to admin commands! (e.g. /admin password)", false)
				default:
					return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("%s is not a recognised command", strings.ToLower(commandSplit[1])), false)
				}
			} else {
				switch strings.ToLower(commandSplit[1]) {
				case "client_list":
					return a.state.SendChat(ServerCarID, entrant.CarID, "See a list of clients in the current entry list", false)
				case "help":
					return a.state.SendChat(ServerCarID, entrant.CarID, "The help command provides context for server commands, just like this!", false)
				case "admin":
					return a.state.SendChat(ServerCarID, entrant.CarID, "The admin command will give you access to admin commands! (e.g. /admin password)", false)
				default:
					return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("%s is not a recognised command, or you do not have access to it", strings.ToLower(commandSplit[1])), false)
				}
			}
		} else {
			if entrant.IsAdmin {
				return errorGroup(
					a.state.SendChat(ServerCarID, entrant.CarID, "Command list: /kick /ban /next_session /restart_session /client_list /ballast /restrictor /help /admin", false),
					a.state.SendChat(ServerCarID, entrant.CarID, "For each command type /help then the command name (e.g. /help kick) for detailed help", false),
					a.state.SendChat(ServerCarID, entrant.CarID, "You have admin permissions on this server", false),
				)
			}

			return errorGroup(
				a.state.SendChat(ServerCarID, entrant.CarID, "Command list: /help /admin", false),
				a.state.SendChat(ServerCarID, entrant.CarID, "For each command type the command name by itself for detailed help", false),
				a.state.SendChat(ServerCarID, entrant.CarID, "You do not have admin permissions on this server", false),
			)
		}
	case "/admin":
		if len(commandSplit) >= 2 {
			if entrant.IsAdmin {
				return a.state.SendChat(ServerCarID, entrant.CarID, "You already have admin permissions!", false)
			}

			if a.state.serverConfig.AdminPassword == strings.Join(commandSplit[1:], " ") {
				entrant.IsAdmin = true

				a.logger.Infof("Admin permissions given to %s (Car ID %d)", entrant.Driver.Name, entrant.CarID)
				a.state.BroadcastChat(ServerCarID, fmt.Sprintf("Admin permissions given to %s!", entrant.Driver.Name), false)
			} else {
				return a.state.SendChat(ServerCarID, entrant.CarID, "Admin password incorrect", false)
			}
		} else {
			return a.state.SendChat(ServerCarID, entrant.CarID, "The admin command will give you access to admin commands! (e.g. /admin password)", false)
		}
	default:
		return a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("%s is not a recognised server command", commandType), false)
	}

	return nil
}
