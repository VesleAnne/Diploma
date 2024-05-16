import json


def send_result(session, result, rabbit_settings):

    """Функция отправки конечного сообщения брокеру
       session - id сессии
       result - непосредственно форматированное сообщение
       rabbit_settings - настройки соединения с rabbitMQ
       Если очередь в брокере не создана,
       автоматически ее создает с указанным в queue_name именем"""
    try:
        queue_name = rabbit_settings[1]
        rabbit_connect = rabbit_settings[0]
        readiness = "error parse message"
        if result != '':
            readiness = "ready"

        data = result
        answer = {
            "sessionId": session,
            "message": readiness,
            "tokens": data
        }

        channel = rabbit_connect.channel()
        channel.queue_declare(queue=queue_name, passive=False, durable=True)

        if result:
            channel.basic_publish(exchange='', routing_key=queue_name, body=json.dumps(answer))
            channel.close()
        return 0
    except Exception as err:
        return err
