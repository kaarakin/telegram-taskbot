package main

import (
	"context"
	"flag"
	"fmt"
	tgbotapi "github.com/skinass/telegram-bot-api/v5"
	"log"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
)

var (
	// токен от @BotFather
	BotToken   = flag.String("tg.token", "", "token for telegram")
	WebhookURL = flag.String("tg.webhook", "", "webhook addr for telegram")

	// мапа для хранения задач
	tasks = make(taskMap)

	// мапа для хранения chatID по юзернеймам
	usersMap = make(map[string]int64)
)

// структура "Задача"
type task struct {
	ID       int
	text     string
	author   string
	assignee string
}

// тип мапа задач
type taskMap map[int]task

// функция для получения слайса задач, отсортированных по ID
func (tasks taskMap) getSortedTasks() []task {
	sortedTasks := make([]task, 0, len(tasks))
	for _, task := range tasks {
		sortedTasks = append(sortedTasks, task)
	}
	sort.Slice(sortedTasks, func(i, j int) bool {
		return sortedTasks[i].ID < sortedTasks[j].ID
	})
	return sortedTasks
}

// функция для получения слайса задач по определенному assignee, отсортированных по ID
func (tasks taskMap) getByAssignee(assignee string) []task {
	tasksByAssignee := make([]task, 0, len(tasks))
	// получение слайса задач, отсортированных по ID
	sortedTasks := tasks.getSortedTasks()
	for _, task := range sortedTasks {
		if task.assignee == assignee {
			tasksByAssignee = append(tasksByAssignee, task)
		}
	}
	return tasksByAssignee
}

// функция для получения слайса задач по определенному author, отсортированных по ID
func (tasks taskMap) getByAuthor(author string) []task {
	tasksByAuthor := make([]task, 0, len(tasks))
	// получение слайса задач, отсортированных по ID
	sortedTasks := tasks.getSortedTasks()
	for _, task := range sortedTasks {
		if task.author == author {
			tasksByAuthor = append(tasksByAuthor, task)
		}
	}
	return tasksByAuthor
}

// функция отправки сообщения от бота
func sendMessage(bot *tgbotapi.BotAPI, chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	_, err := bot.Send(msg)
	if err != nil {
		log.Printf("Ошибка при отправке сообщения: %s\n", err)
	}
}

// функция для вывода списка задач
func showTasks(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	// получение слайса задач, отсортированных по ID
	tasksSorted := tasks.getSortedTasks()

	if len(tasksSorted) == 0 {
		sendMessage(bot, update.Message.Chat.ID, "Нет задач")
		return
	}

	answers := make([]string, 0)
	for _, task := range tasksSorted {
		var taskMessage, answer string

		taskMessage = fmt.Sprintf("%v. %s by @%s", task.ID, task.text, task.author)
		if task.assignee == "" {
			answer = taskMessage + fmt.Sprintf("\n/assign_%v", task.ID)
		}
		if task.assignee == update.Message.From.String() {
			answer = taskMessage + fmt.Sprintf("\nassignee: я\n/unassign_%v /resolve_%v", task.ID, task.ID)
		}
		if task.assignee != update.Message.From.String() && task.assignee != "" {
			answer = taskMessage + fmt.Sprintf("\nassignee: @%s", task.assignee)
		}
		answers = append(answers, answer)
	}
	sendMessage(bot, update.Message.Chat.ID, strings.Join(answers, "\n\n"))
}

// функция для вывода списка задач, где assignee - это отправитель команды
func showMyTasks(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	tasksSorted := tasks.getByAssignee(update.Message.From.String())
	if len(tasksSorted) == 0 {
		sendMessage(bot, update.Message.Chat.ID, "Нет задач")
		return
	}

	answers := make([]string, 0)
	for _, task := range tasksSorted {
		var taskMessage, answer string

		// возвращаются только те задачи, где assignee - это отправитель команды
		if task.assignee == update.Message.From.String() {
			taskMessage = fmt.Sprintf("%v. %s by @%s", task.ID, task.text, task.author)
			answer = taskMessage + fmt.Sprintf("\n/unassign_%v /resolve_%v", task.ID, task.ID)
			answers = append(answers, answer)
		}
	}
	sendMessage(bot, update.Message.Chat.ID, strings.Join(answers, ""))
}

// функция для вывода списка задач, где author - это отправитель команды
func showOwnTasks(bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	tasksSorted := tasks.getByAuthor(update.Message.From.String())
	if len(tasksSorted) == 0 {
		sendMessage(bot, update.Message.Chat.ID, "Нет задач")
		return
	}

	answers := make([]string, 0)

	for _, task := range tasksSorted {
		var taskMessage, answer string

		// возвращаются только те задачи, где author - это отправитель команды
		if task.author == update.Message.From.String() {
			taskMessage = fmt.Sprintf("%v. %s by @%s", task.ID, task.text, task.author)
			if task.assignee != update.Message.From.String() {
				answer = taskMessage + fmt.Sprintf("\n/assign_%v", task.ID)
				answers = append(answers, answer)
			} else {
				answers = append(answers, taskMessage)
			}
		}
	}
	sendMessage(bot, update.Message.Chat.ID, strings.Join(answers, ""))
}

// функция создания новой задачи
func createTask(bot *tgbotapi.BotAPI, update tgbotapi.Update, taskID *int, text string) {
	// инкремент идентификатора задачи
	*taskID++
	tasks[*taskID] = task{ID: *taskID, text: text, author: update.Message.From.String(), assignee: ""}
	sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf(`Задача "%s" создана, id=%v`, tasks[*taskID].text, tasks[*taskID].ID))
}

// функция для назначения задачи на пользователя
func assignTask(bot *tgbotapi.BotAPI, update tgbotapi.Update, taskID int) {
	task, ok := tasks[taskID]
	if !ok {
		sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf("Задача с id=%v не существует", taskID))
	}

	previousAssignee := tasks[taskID].assignee
	task.assignee = update.Message.From.String()
	tasks[taskID] = task

	sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf(`Задача "%s" назначена на вас`, tasks[taskID].text))

	if previousAssignee == "" && tasks[taskID].author != update.Message.From.String() {
		authorID, ok := usersMap[tasks[taskID].author]
		if ok {
			sendMessage(bot, authorID, fmt.Sprintf(`Задача "%s" назначена на @%s`, tasks[taskID].text, tasks[taskID].assignee))
		}
	}

	if previousAssignee != "" {
		previousAssigneeChatID, ok := usersMap[previousAssignee]
		if ok {
			sendMessage(bot, previousAssigneeChatID, fmt.Sprintf(`Задача "%s" назначена на @%s`, tasks[taskID].text, tasks[taskID].assignee))
		}
	}
}

// функция для снятия пользователя с задачи
func unassignTask(bot *tgbotapi.BotAPI, update tgbotapi.Update, taskID int) {
	task, ok := tasks[taskID]
	if !ok {
		sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf("Задача с id=%v не существует", taskID))
	}

	if tasks[taskID].assignee != update.Message.From.String() {
		sendMessage(bot, update.Message.Chat.ID, "Задача не на вас")
		return
	}

	if tasks[taskID].assignee == update.Message.From.String() {
		task.assignee = ""
		tasks[taskID] = task

		sendMessage(bot, update.Message.Chat.ID, "Принято")

		authorID, ok := usersMap[tasks[taskID].author]
		if ok {
			sendMessage(bot, authorID, fmt.Sprintf(`Задача "%s" осталась без исполнителя`, tasks[taskID].text))
		}
	}
}

// функция для выполнения задачи и удаления ее из базы
func resolveTask(bot *tgbotapi.BotAPI, update tgbotapi.Update, taskID int) {
	_, ok := tasks[taskID]
	if !ok {
		sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf("Задача с id=%v не существует", taskID))
	}

	if tasks[taskID].assignee != update.Message.From.String() {
		sendMessage(bot, update.Message.Chat.ID, "Задача не на вас")
	}

	if tasks[taskID].assignee == update.Message.From.String() {
		sendMessage(bot, update.Message.Chat.ID, fmt.Sprintf(`Задача "%s" выполнена`, tasks[taskID].text))

		if tasks[taskID].author != update.Message.From.String() {
			sendMessage(bot, usersMap[tasks[taskID].author], fmt.Sprintf(`Задача "%s" выполнена @%s`, tasks[taskID].text, tasks[taskID].assignee))
		}

		delete(tasks, taskID)
	}
}

// функция, обрабатывающая команды пользователя
func handleCommand(bot *tgbotapi.BotAPI, update tgbotapi.Update, command string, taskID *int) {
	parsedCommand := strings.Split(command, " ")
	action := parsedCommand[0]

	switch {
	// команда просмотра задач
	case action == "/tasks":
		if len(parsedCommand) > 1 {
			sendMessage(bot, update.Message.Chat.ID, "Команда '/tasks' не требует аргументов")
			return
		}

		showTasks(bot, update)

	// команда создания новой задачи
	case action == "/new":
		if len(parsedCommand) == 1 {
			sendMessage(bot, update.Message.Chat.ID, "Нельзя создать пустую задачу")
			return
		}
		// taskID++
		createTask(bot, update, taskID, strings.Join(parsedCommand[1:], " "))

	// команда назначения задачи на пользователя
	case strings.Contains(action, "/assign_"):
		taskID, err := strconv.Atoi(action[8:])
		if err != nil {
			sendMessage(bot, update.Message.Chat.ID, "Некорректное значение id задачи")
		}

		assignTask(bot, update, taskID)

	// команда снятия задачи с пользователя
	case strings.Contains(action, "/unassign_"):
		taskID, err := strconv.Atoi(action[10:])
		if err != nil {
			sendMessage(bot, update.Message.Chat.ID, "Некорректное значение id задачи")
		}

		unassignTask(bot, update, taskID)

	case strings.Contains(action, "/resolve_"):
		taskID, err := strconv.Atoi(action[9:])

		if err != nil {
			sendMessage(bot, update.Message.Chat.ID, "Некорректное значение id задачи")
		}

		resolveTask(bot, update, taskID)
	// команда просмотра задач, назначенных на пользователя
	case action == "/my":
		if len(parsedCommand) > 1 {
			sendMessage(bot, update.Message.Chat.ID, "Команда '/my' не требует аргументов")
			return
		}

		showMyTasks(bot, update)

	// команда просмотра задач пользователя, авторами которых он является
	case action == "/owner":
		if len(parsedCommand) > 1 {
			sendMessage(bot, update.Message.Chat.ID, "Команда '/owner' не требует аргументов")
			return
		}

		showOwnTasks(bot, update)

	default:
		sendMessage(bot, update.Message.Chat.ID, "Неизвестная команда")
		return
	}
}

// функция старта работы бота
func startTaskBot(ctx context.Context) error {
	// идентификатор задачи
	var taskID = new(int)
	flag.Parse()

	bot, err := tgbotapi.NewBotAPI(*BotToken)
	if err != nil {
		return fmt.Errorf("NewBotAPI failed: %s", err)
	}

	bot.Debug = true
	fmt.Printf("Authorized on account %s\n", bot.Self.UserName)

	wh, err := tgbotapi.NewWebhook(*WebhookURL)
	if err != nil {
		return fmt.Errorf("NewWebhook failed: %s", err)
	}

	_, err = bot.Request(wh)
	if err != nil {
		return fmt.Errorf("SetWebhook failed: %s", err)
	}

	updates := make(chan tgbotapi.Update)
	go func() {
		for update := range bot.ListenForWebhook("/") {
			select {
			case <-ctx.Done():
				close(updates)
				return
			case updates <- update:
			}
		}
	}()

	http.HandleFunc("/state", func(w http.ResponseWriter, r *http.Request) {
		_, err := w.Write([]byte("all is working"))
		if err != nil {
			fmt.Printf("Failed to write response: %s\n", err)
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	go func() {
		log.Fatalln("http err:", http.ListenAndServe(":"+port, nil))
	}()
	fmt.Println("start listen :" + port)

	for update := range updates {
		// обработка обновления
		log.Printf("upd: %#v\n", update)

		// добавление Chat.ID в мапу сопоставления юзернеймов и Chat.ID
		usersMap[update.Message.From.String()] = update.Message.Chat.ID
		// передача команды в обработчик команд
		command := update.Message.Text
		handleCommand(bot, update, command, taskID)
	}

	// возвращение ошибки, если возникнет ошибка в основном цикле
	return nil
}

func main() {
	err := startTaskBot(context.Background())
	if err != nil {
		panic(err)
	}
}

