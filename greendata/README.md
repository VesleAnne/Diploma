# Чат-бот для поддержки клиентов Greendata

1. Скачать модель,токенизатор и faq с гугл драйва ([Скачать)](https://drive.google.com/drive/folders/19TT3zJde24Wy4lY8GyWUsmpZ0Wie_3Dy?usp=drive_link)
2. Поместить в папку models
2. docker-compose up



**POST /api/send** - отправка пакета данных для генерации ответа
- Параметры: нет
- Тело запроса: JSON с пакетом данных согласно спецификации
например,
```json
{
  "sessionId": "1",
  "data": [
    {
      "id": 0,
      "string": "Какие могут быть траифы"
    },
    {
      "id": 1,
      "string": "Как создать БД"
    }
  ]
}
```

**POST /api/send/file** - отправка файла с данными для а генерации файлов
- Параметры: нет
- Тело запроса: набор записей (строк) в простом текстовом формате

> Каждая запись должна оканчиваться переводом строки ("\n","\r","\r\n"). Пустые строки игнорируются.

например:
```
Какие могут быть траифы
Как создать БД
```


- Ответ: JSON со сгенерированным SID
```json
{
    "message": "data sent for processing",
    "sid": "6313d7d3-7559-11ed-b6da-0242ac120005"
}
```
Пример запроса для отправки файла с использованием curl:

`$ curl -X POST --data-binary @test.txt http://localhost:3001/api/send/file`

**GET /api/get** - получение всех результатов
- Параметры: нет
- Тело запроса: нет
- Ответ: JSON с результатами обработанных строк


**GET /api/get/\{sid\}** - получение результатов по конкретному SessionID
- Параметры: \{sid\} - обязательный - string
- Тело запроса: нет
- Ответ: JSON с результатами обработанных строк для конкретного SessionID
например,