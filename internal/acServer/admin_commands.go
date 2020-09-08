package acServer

import (
	"fmt"
	"strconv"
	"strings"
)

type AdminCommandManager struct {
	state          *ServerState
	sessionManager *SessionManager
	logger         Logger
}

func NewAdminCommandManager(state *ServerState, sessionManager *SessionManager, logger Logger) *AdminCommandManager {
	return &AdminCommandManager{
		state:          state,
		sessionManager: sessionManager,
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
				a.state.SendChat(ServerCarID, commandEntrant.CarID, fmt.Sprintf("Could not find entrant %s", name))
				return nil
			}
		}
	} else {
		entrantToReturn, _ = a.state.GetCarByID(CarID(carIDToReturn))
	}

	if entrantToReturn == nil {
		// try getting entrant by guid
		entrantToReturn = a.state.GetCarByGUID(commandSplit[1], true)

		if entrantToReturn == nil {
			a.state.SendChat(ServerCarID, commandEntrant.CarID, fmt.Sprintf("Could not find entrant %s", commandSplit[1]))
			return nil
		}
	}

	if !entrantToReturn.IsConnected() {
		a.state.SendChat(ServerCarID, commandEntrant.CarID, fmt.Sprintf("Car %d is not connected to the server", entrantToReturn.CarID))
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
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /kick command! Use /admin to get permission")
			return nil
		}

		if len(commandSplit) >= 2 {
			entrantToKick := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToKick != nil {
				a.state.Kick(entrantToKick.CarID, KickReasonGeneric)
			}
		} else {
			a.state.SendChat(ServerCarID, entrant.CarID, "Kick commands require the car ID, GUID or name to be kicked! (e.g. /kick 3)")
		}
	case "/ban", "/ban_id":
		if !entrant.IsAdmin {
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /ban command! Use /admin to get permission")
			return nil
		}

		if len(commandSplit) >= 2 {
			entrantToBan := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToBan != nil {
				a.state.Kick(entrantToBan.CarID, KickReasonGeneric)
				err := a.state.AddToBlockList(entrantToBan.Driver.GUID)

				if err != nil {
					a.logger.WithError(err).Errorf("Couldn't add %s to the server blocklist.json", entrantToBan.Driver.GUID)
					a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Couldn't add %s to the server blocklist.json", entrantToBan.Driver.Name))
				} else {
					a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Successfully added %s to the server blocklist.json", entrantToBan.Driver.Name))
				}
			}
		} else {
			a.state.SendChat(ServerCarID, entrant.CarID, "Ban commands require the car ID, GUID or name to be kicked! (e.g. /ban 3)")
		}
	case "/next_session":
		if !entrant.IsAdmin {
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /next_session command! Use /admin to get permission")
			return nil
		}

		a.sessionManager.NextSession(true)
		a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s instructed the server to change to the next session", entrant.Driver.Name))
	case "/restart_session":
		if !entrant.IsAdmin {
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /restart_session command! Use /admin to get permission")
			return nil
		}

		a.sessionManager.RestartSession()
		a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s instructed the server to restart the session", entrant.Driver.Name))
	case "/ballast":
		if !entrant.IsAdmin {
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /ballast command! Use /admin to get permission")
			return nil
		}

		if len(commandSplit) >= 2 {
			entrantToApplyBallast := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToApplyBallast != nil {
				ballastString := commandSplit[len(commandSplit)-1]

				ballast, err := strconv.ParseFloat(ballastString, 32)

				if err != nil {
					a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Could not parse ballast %s as a number!", ballastString))
					return nil
				}

				if ballast > 5000 {
					ballast = 5000
				}

				if ballast < 0 {
					ballast = 0
				}

				entrantToApplyBallast.Ballast = float32(ballast)

				a.state.BroadcastUpdateBoP(entrantToApplyBallast)

				a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has set %s's ballast to %.0fkg!", entrant.Driver.Name, entrantToApplyBallast.Driver.Name, ballast))
			}
		} else {
			a.state.SendChat(ServerCarID, entrant.CarID, "ballast commands require the car ID, GUID or name and ballast amount! (e.g. /ballast 5 80)")
		}
	case "/restrictor":
		if !entrant.IsAdmin {
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /restrictor command! Use /admin to get permission")
			return nil
		}

		if len(commandSplit) >= 2 {
			entrantToApplyRestrictor := a.GetEntrantFromCommandSplit(commandSplit, entrant)

			if entrantToApplyRestrictor != nil {
				restrictorString := commandSplit[len(commandSplit)-1]

				restrictor, err := strconv.ParseFloat(restrictorString, 32)

				if err != nil {
					a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("Could not parse restrictor %s as a number!", restrictorString))
					return nil
				}

				if restrictor > 400 {
					restrictor = 400
				}

				if restrictor < 0 {
					restrictor = 0
				}

				entrantToApplyRestrictor.Restrictor = float32(restrictor)

				a.state.BroadcastUpdateBoP(entrantToApplyRestrictor)

				a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has set %s's restrictor to %.0f%%!", entrant.Driver.Name, entrantToApplyRestrictor.Driver.Name, restrictor))
			}
		} else {
			a.state.SendChat(ServerCarID, entrant.CarID, "restrictor commands require the car ID, GUID or name and restrictor amount! (e.g. /ballast 5 80)")
		}
	case "/next_weather":
		if !entrant.IsAdmin {
			a.state.SendChat(ServerCarID, entrant.CarID, "Only admins can use the /next_weather command! Use /admin to get permission")
			return nil
		}

		if a.sessionManager.weatherProgression {
			a.state.BroadcastChat(ServerCarID, fmt.Sprintf("%s has changed the weather to the next configured weather!", entrant.Driver.Name))

			a.sessionManager.NextWeather(currentTimeMillisecond())
		} else {
			a.state.SendChat(ServerCarID, entrant.CarID, "This session does not have weather progression enabled! Look at the readme for more info+")
		}
	case "/help":
		if len(commandSplit) == 2 {
			if entrant.IsAdmin {
				switch strings.ToLower(commandSplit[1]) {
				case "kick":
					a.state.SendChat(ServerCarID, entrant.CarID, "Kick a driver from the server using car ID, GUID or name! (e.g. /kick 3)")
				case "ban":
					a.state.SendChat(ServerCarID, entrant.CarID, "Kick a driver from the server and add them to the block list using car ID, GUID or name! (e.g. /kick 3)")
				case "next_session":
					a.state.SendChat(ServerCarID, entrant.CarID, "Move to the next configured session, or back to the first session if loop mode is on")
				case "restart_session":
					a.state.SendChat(ServerCarID, entrant.CarID, "Restart the current session")
				case "client_list":
					a.state.SendChat(ServerCarID, entrant.CarID, "See a list of clients in the current entry list")
				case "ballast":
					a.state.SendChat(ServerCarID, entrant.CarID, "Apply ballast (maximum 5000kg) to a driver from the server using car ID, GUID or name! (e.g. /ballast Kevin 40)")
				case "restrictor":
					a.state.SendChat(ServerCarID, entrant.CarID, "Apply an air intake restrictor (maximum 400%) to a driver from the server using car ID, GUID or name! (e.g. /restrictor Brad 40)")
				case "next_weather":
					a.state.SendChat(ServerCarID, entrant.CarID, "Move to the next configured weather in the session")
				case "help":
					a.state.SendChat(ServerCarID, entrant.CarID, "The help command provides context for server commands, just like this!")
				case "admin":
					a.state.SendChat(ServerCarID, entrant.CarID, "The admin command will give you access to admin commands! (e.g. /admin password)")
				default:
					a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("%s is not a recognised command", strings.ToLower(commandSplit[1])))
				}
			} else {
				switch strings.ToLower(commandSplit[1]) {
				case "client_list":
					a.state.SendChat(ServerCarID, entrant.CarID, "See a list of clients in the current entry list")
				case "help":
					a.state.SendChat(ServerCarID, entrant.CarID, "The help command provides context for server commands, just like this!")
				case "admin":
					a.state.SendChat(ServerCarID, entrant.CarID, "The admin command will give you access to admin commands! (e.g. /admin password)")
				default:
					a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("%s is not a recognised command, or you do not have access to it", strings.ToLower(commandSplit[1])))
				}
			}
		} else {
			if entrant.IsAdmin {
				a.state.SendChat(ServerCarID, entrant.CarID, "Command list: /kick /ban /next_session /restart_session /client_list /ballast /restrictor /help /admin")
				a.state.SendChat(ServerCarID, entrant.CarID, "For each command type /help then the command name (e.g. /help kick) for detailed help")
				a.state.SendChat(ServerCarID, entrant.CarID, "You have admin permissions on this server")
			} else {
				a.state.SendChat(ServerCarID, entrant.CarID, "Command list: /help /admin")
				a.state.SendChat(ServerCarID, entrant.CarID, "For each command type the command name by itself for detailed help")
				a.state.SendChat(ServerCarID, entrant.CarID, "You do not have admin permissions on this server")
			}
		}
	case "/admin":
		if len(commandSplit) >= 2 {
			if entrant.IsAdmin {
				a.state.SendChat(ServerCarID, entrant.CarID, "You already have admin permissions!")
				return nil
			}

			if a.state.serverConfig.AdminPassword == strings.Join(commandSplit[1:], " ") {
				entrant.IsAdmin = true

				a.logger.Infof("Admin permissions given to %s (Car ID %d)", entrant.Driver.Name, entrant.CarID)
				a.state.BroadcastChat(ServerCarID, fmt.Sprintf("Admin permissions given to %s!", entrant.Driver.Name))
			} else {
				a.state.SendChat(ServerCarID, entrant.CarID, "Admin password incorrect")
			}
		} else {
			a.state.SendChat(ServerCarID, entrant.CarID, "The admin command will give you access to admin commands! (e.g. /admin password)")
		}
	default:
		a.state.SendChat(ServerCarID, entrant.CarID, fmt.Sprintf("%s is not a recognised server command", commandType))
	}

	return nil
}
