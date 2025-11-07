package main


import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"os"
	"regexp"
	"strconv"
	"errors"
	tgbotapi "github.com/skinass/telegram-bot-api/v5"
)

type Task struct {
	Id int64
	Description string
	Owner string
	Assignee string
	IsAssign bool
	OwnerChatId int64
	AssigneeChatId int64
}


var tasks []Task
var idCount int64

var (
	BotToken = *flag.String("tg.token", "", "token for telegram")
	WebhookURL = *flag.String("tg.webhook", "", "webhook addr for telegram")
)

func getTasks (chatId int64) string {
	var msgString string
	if (len(tasks) == 0) {
		msgString += "Нет задач"
		return msgString
	}
	for idx, task := range tasks {
		if idx != 0 {
			msgString += "\n\n"
		}

		msgString += fmt.Sprintf("%d. %s by @%s\n", task.Id, task.Description, task.Owner)
		if (task.IsAssign && task.AssigneeChatId == chatId) {
			msgString += fmt.Sprintf("assignee: я\n/unassign_%d /resolve_%d", 
			task.Id, task.Id)	
		} else if (task.IsAssign && task.AssigneeChatId != chatId) {
			msgString += fmt.Sprintf("assignee: @%s", task.Assignee)
		} else {
			msgString += fmt.Sprintf("/assign_%d", task.Id)
		}
	} 
	return msgString
} 

func appendNewTask(description string, owner string, chatId int64) ([]Task, string) {
	idCount++
	newTask := Task {
		Id: idCount,
		Description: description,
		Owner: owner,
		OwnerChatId: chatId,
	}

	tasks = append(tasks, newTask)

	response := fmt.Sprintf(`Задача "%s" создана, id=%d`, newTask.Description, newTask.Id)

	return tasks, response
}

func findTask(id int64) (Task, bool) {
	for _, task := range tasks {
		if (task.Id == id) {
			return task, true
		}
	}
	return Task{}, false
}

func updateTask(id int64, updatedTask Task, needDelete bool) {
	for i := range tasks {
		if (tasks[i].Id == id) {
			if (needDelete) {
				newTasks := make([]Task, 0)
				newTasks = append(newTasks, tasks[:i]...)
				newTasks = append(newTasks, tasks[i+1:]...)
				tasks = newTasks
				return
			}
			tasks[i] = updatedTask
		}
	}
}

func assignTask(command string, assigner string, assignerChatId int64) (map[int64]string, error) {
	id, err := strconv.Atoi(command)

	msgToDifferentChats := make(map[int64]string)

	if (err != nil) {
		err := errors.New("internal server error")
		return map[int64]string{}, err
	}

	task, finded := findTask(int64(id))

	if (!finded) {
		err := errors.New("unknown task")
		return map[int64]string{}, err
	}

	updatedTask := task
	updatedTask.IsAssign = true

	msgStringToAssigner := fmt.Sprintf(`Задача "%s" назначена на вас`, task.Description)
	msgToDifferentChats[assignerChatId] = msgStringToAssigner
	updatedTask.Assignee = assigner
	updatedTask.AssigneeChatId = assignerChatId

	if (task.Assignee != "") {
		msgStringToPrevAssignee := fmt.Sprintf(`Задача "%s" назначена на @%s`, task.Description, assigner)
		msgToDifferentChats[task.AssigneeChatId] = msgStringToPrevAssignee
	} else if (task.Owner != assigner) {
		msgStringToOwner := fmt.Sprintf(`Задача "%s" назначена на @%s`, task.Description, assigner)
		msgToDifferentChats[task.OwnerChatId] = msgStringToOwner
	}

	updateTask(task.Id, updatedTask, false)

	return msgToDifferentChats, nil
}

func unassignTask(command string, assignee string, chatId int64) (map[int64]string, error) {
	msgToDifferentChats := make(map[int64]string)
	
	id, err := strconv.Atoi(command)
	if (err != nil) {
		err := errors.New("internal server error")
		return map[int64]string{}, err
	}
	
	task, finded := findTask(int64(id))
	
	if (!finded) {
		err := errors.New("unknown task")
		return	map[int64]string{}, err

	}
	if (task.Assignee == assignee && task.AssigneeChatId == chatId) {
		task.Assignee = ""
		task.IsAssign = false
		msgToDifferentChats[chatId] = "Принято"
		
		if (task.OwnerChatId != chatId) {
			msgToDifferentChats[task.OwnerChatId] = fmt.Sprintf(`Задача "%s" осталась без исполнителя`, task.Description)
		}
	} else if (task.Assignee != assignee) {
		msgToDifferentChats[chatId] = "Задача не на вас"
	}


	updateTask(int64(id), task, false)
	
	return msgToDifferentChats, nil
}

func resolveTask(command string, assignee string, chatId int64) (map[int64]string, error) {
	msgToDifferentChats := make(map[int64]string)
	
	id, err := strconv.Atoi(command)
	if (err != nil) {
		err := errors.New("internal server error")
		return map[int64]string{}, err
	}
	
	task, finded := findTask(int64(id))
	
	if (!finded) {
		err := errors.New("unknown task")
		return	map[int64]string{}, err

	}

	if (task.Assignee == assignee && task.AssigneeChatId == chatId) {
		updateTask(int64(id), task, true)
		msgToDifferentChats[chatId] = fmt.Sprintf(`Задача "%s" выполнена`, task.Description)
	} else if (task.Assignee != assignee) {
		msgToDifferentChats[chatId] = "Задача не на вас"
	}
	
	if (task.OwnerChatId != chatId) {
		msgToDifferentChats[task.OwnerChatId] = fmt.Sprintf(`Задача "%s" выполнена @%s`, task.Description, task.Assignee)
	} 

	
	return msgToDifferentChats, nil
}

func getMyTasks(assignee string) string {
	var msgString string
	for _, task := range tasks {
		if (task.Assignee == assignee) {
			msgString += fmt.Sprintf("%d. %s by @%s\n", task.Id, task.Description, task.Owner)
			msgString += fmt.Sprintf("/unassign_%d /resolve_%d", 
			task.Id, task.Id)	
		}
	}
	return msgString
}

func getMyOwnTasks(owner string, chatId int64) string {
	msgString := ""
	idx := 0
	for _, task := range tasks {
		if (task.Owner == owner) {
			if idx != 0 {
				msgString += "\n\n"
			}
	
			msgString += fmt.Sprintf("%d. %s by @%s\n", task.Id, task.Description, task.Owner)
			if (task.IsAssign && task.AssigneeChatId == chatId) {
				msgString += fmt.Sprintf("assignee: я\n/unassign_%d /resolve_%d", 
				task.Id, task.Id)	
			} else if (task.IsAssign && task.AssigneeChatId != chatId) {
				msgString += fmt.Sprintf("assignee: @%s", task.Assignee)
			} else {
				msgString += fmt.Sprintf("/assign_%d", task.Id)
			}
			idx++
		} 
		}
	return msgString
}

func startTaskBot(ctx context.Context) error {
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(BotToken)
	if err != nil {
		log.Fatalf("NewBotApi failed: %s", err)
	}

	bot.Debug = true
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	wh, err := tgbotapi.NewWebhook(WebhookURL)
	if err != nil {
		log.Fatalf("NewWebhook failed: %s", err)
	}

	_, err = bot.Request(wh)

	if (err != nil) {
		log.Fatalf("SetWebHook failed: %s", err)
	}

	updates := bot.ListenForWebhook("/")

	http.HandleFunc("/state", func(w http.ResponseWriter, r* http.Request) {
		w.Write([]byte("all is working"))
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	go func() {
		log.Fatalln("http err:", http.ListenAndServe(":"+port, nil))
	} ()

	fmt.Println("start listen :" + port)

	for update := range updates {
		log.Printf("upd: %#v\n", update)
		if update.Message != nil {
			log.Printf("upd: %v\n", update)
			text := update.Message.Text
			var msgString string
			switch {
			case	strings.Contains(text, "/tasks"):
				if len(text) > len("/tasks") {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "invalid command")
					bot.Send(msg)
					continue
				}
				msgString = getTasks(update.Message.Chat.ID)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgString)
				bot.Send(msg)

			case	strings.Contains(text, "/new"):
				if (len(text[(len("/new") - 1):]) == 0 || text[len("/new")] != ' ') {
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, "invalid command")
					bot.Send(msg)
				}
				tasks, msgString = appendNewTask(text[len("/new")+1:], update.Message.From.UserName, update.Message.Chat.ID)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgString)
				bot.Send(msg)
			case	strings.Contains(text, "/assign"), strings.Contains(text, "/unassign"), strings.Contains(text, "/resolve"):
				reg := regexp.MustCompile(`^/(resolve|assign|unassign)_(\d+)$`)
				matches := reg.FindStringSubmatch(text)
				if (matches == nil) {
					msgString = "Invalid command"
					msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgString)
					bot.Send(msg)
					continue
				}
				
				switch matches[1]{
				case "assign":
					msgToDifferentChats, err := assignTask(matches[2], update.Message.From.UserName, update.Message.Chat.ID)
					if err != nil {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
						bot.Send(msg)
						continue
					}
	
					for chatId, msgString := range msgToDifferentChats {
						msg := tgbotapi.NewMessage(chatId, msgString)
						bot.Send(msg)
					}
				case "unassign":
					msgToDifferentChats, err := unassignTask(matches[2], update.Message.From.UserName, update.Message.Chat.ID)
					if (err != nil) {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
						bot.Send(msg)
						continue
					}
	
					for chatId, msgString := range msgToDifferentChats {
						msg := tgbotapi.NewMessage(chatId, msgString)
						bot.Send(msg)
					}
				case "resolve":
					msgToDifferentChats, err := resolveTask(matches[2], update.Message.From.UserName, update.Message.Chat.ID)
					if (err != nil) {
						msg := tgbotapi.NewMessage(update.Message.Chat.ID, err.Error())
						bot.Send(msg)
						continue
					}

					for chatId, msgString := range msgToDifferentChats {
						msg := tgbotapi.NewMessage(chatId, msgString)
						bot.Send(msg)
					}
				}
			case	strings.Contains(text, "/my"):
				msgString = getMyTasks(update.Message.From.UserName)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgString)
				bot.Send(msg)
			case	strings.Contains(text, "/owner"):
				msgString = getMyOwnTasks(update.Message.From.UserName, update.Message.Chat.ID)
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, msgString)
				bot.Send(msg)
			default:
				msg := tgbotapi.NewMessage(update.Message.Chat.ID, 
				"unknown command")
				bot.Send(msg)
			}
		}
		
	}
	return nil
}

func main() {
	err := startTaskBot(context.Background())
	if err != nil {
		panic(err)
	}
}