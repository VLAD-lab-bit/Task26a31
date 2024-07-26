package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"sync"
	"time"
)

const (
	BufferSize = 5
	Interval   = 5 * time.Second
)

// Потокобезопасная версия стадии конвейера, фильтрующая отрицательные числа
func filterNegative(done <-chan struct{}, input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for {
			select {
			case <-done:
				return
			case i, isChannelOpen := <-input:
				if !isChannelOpen {
					return
				}
				if i >= 0 {
					output <- i
				}
			}
		}
	}()
	return output
}

// Потокобезопасная версия стадии конвейера, фильтрующая числа, не кратные 3
func filterNonMultipleOfThree(done <-chan struct{}, input <-chan int) <-chan int {
	output := make(chan int)
	go func() {
		defer close(output)
		for {
			select {
			case <-done:
				return
			case i, isChannelOpen := <-input:
				if !isChannelOpen {
					return
				}
				if i != 0 && i%3 == 0 {
					output <- i
				}
			}
		}
	}()
	return output
}

// Кольцевой буфер
type RingBuffer struct {
	data    []int
	maxSize int
	nextIn  int
	nextOut int
	count   int
	mu      sync.Mutex
}

// Создание нового кольцевого буфера заданного размера
func NewRingBuffer(maxSize int) *RingBuffer {
	return &RingBuffer{
		data:    make([]int, maxSize),
		maxSize: maxSize,
		nextIn:  0,
		nextOut: 0,
		count:   0,
	}
}

// Добавление элемента в буфер с проверкой переполнения
func (rb *RingBuffer) Push(val int) bool {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	if rb.count == rb.maxSize { // Проверка на переполнение
		return false // Возвращаем false, если буфер полон
	}
	rb.data[rb.nextIn] = val
	rb.nextIn = (rb.nextIn + 1) % rb.maxSize
	rb.count++
	return true // Возвращаем true, если добавление прошло успешно
}

// Извлечение элемента из буфера
func (rb *RingBuffer) Pop() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	val := rb.data[rb.nextOut]
	rb.nextOut = (rb.nextOut + 1) % rb.maxSize
	rb.count--
	return val
}

// Количество элементов в буфере
func (rb *RingBuffer) Count() int {
	rb.mu.Lock()
	defer rb.mu.Unlock()
	return rb.count
}

// Источник данных для конвейера
func dataSource(done chan<- struct{}) <-chan int {
	output := make(chan int)
	scanner := bufio.NewScanner(os.Stdin)
	go func() {
		defer close(output)
		for scanner.Scan() {
			input := scanner.Text()
			if input == "exit" {
				close(done) // Отправляем сигнал для завершения
				return
			}
			num, err := strconv.Atoi(input)
			if err == nil {
				output <- num
			} else {
				fmt.Println("Введено нечисловое значение, игнорируется:", input)
			}
		}
	}()
	return output
}

func dataConsumer(done <-chan struct{}, input <-chan int, bufferSize int, interval time.Duration) {
	buffer := NewRingBuffer(bufferSize)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case val, isOpen := <-input:
			if !isOpen {
				return
			}
			if !buffer.Push(val) {
				fmt.Println("Буфер переполнен, значение игнорируется:", val)
			}
		case <-ticker.C:
			for buffer.Count() > 0 {
				fmt.Println("Получены данные:", buffer.Pop())
			}
		}
	}
}

func main() {
	done := make(chan struct{}) // Используем пустую структуру для сигнального канала

	pipeline := filterNonMultipleOfThree(done, filterNegative(done, dataSource(done)))

	go dataConsumer(done, pipeline, BufferSize, Interval)

	// Ожидание сигнала для завершения
	<-done
	fmt.Println("Программа завершила работу.")
}
