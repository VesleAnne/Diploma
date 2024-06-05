import os
import json
import time
import datetime
import torch
import pika
import redis
import pandas as pd
import pickle

import scripts.QA
from scripts.redis_workflow import redis_connection, import_from_redis
from scripts.send_message import send_result



def log_file(error_type):
    """Функция логирования работы программы
    И контроля файла log.txt
    По умолчанию размер файла = 2МБ"""

    try:
        time_now = datetime.datetime.now().strftime('%H:%M:%S')
        max_size_log = 2000000
        now_size_log = os.stat('log.txt').st_size
        print(f'{time_now} {error_type}')

        if now_size_log > max_size_log:
            with open("log.txt", 'r', encoding='utf-8') as file:
                save_head = [next(file) for x in range(10)]
            os.remove("log.txt")
            with open("log.txt", 'w', encoding='utf-8') as file:
                for line in save_head:
                    file.write(str(line))
                file.write("..............\n")

    except FileNotFoundError:
        with open("log.txt", 'w', encoding='utf-8') as file:
            file.write(f'{time_now} Новый файл логирования создан \n')
    with open("log.txt", 'a', encoding='utf-8') as file:
        write_str = time_now + " " + error_type + "\n"
        file.write(write_str)


def config_init():
    """ Функция для загрузки конфигурации from .env
       QA_MODEL - путь к nn модели НС
       FAQ_EMBEDDINGS - путь к файлу для создания эмбендингов
       OUT_FILE_PATH - путь к выходному файлу с выводом ошибок
       REDIS_HOST - хост для подключения Redis
       REDIS_PORT - порт для подключения Redis
       SAVE_FILE - флаг сохранения файла работы xls
       """
    batch_size = os.getenv('CHUNK_SIZE')
    if batch_size is None or batch_size == '':
        print("Значение переменной CHUNK_SIZE "
              "не установлено, используется значение по умолчанию")
        batch_size = 25000
    print('CHUNK_SIZE = ', batch_size)
    gpu_status = os.getenv('USE_GPU')
    if gpu_status is None or gpu_status == '':
        print("Значение переменной USE_GPU "
              "не установлено, используется значение по умолчанию")
        gpu_status = 'True'
    output_file = os.getenv('OUT_FILE_PATH')
    if output_file is None or output_file == '':
        print("Значение переменной OUT_FILE_PATH "
              "не установлено, используется значение по умолчанию")
        output_file = './errors'
    save_file = os.getenv('SAVE_FILE')
    if save_file is None or save_file == '':
        print("Значение переменной SAVE_FILE "
              "не установлено, используется значение по умолчанию")
        save_file = '1'
    QA_model_pkl = os.getenv('QA_MODEL')
    if QA_model_pkl is None or QA_model_pkl == '':
        print("Значение переменной QA_MODEL "
              "не установлено, используется значение по умолчанию")
        QA_model_pkl = './model/tokenizer_and_model.pkl'
    redis_host = os.getenv('REDIS_HOST')
    if redis_host is None or redis_host == '':
        print("Значение переменной REDIS_HOST "
              "не установлено, используется значение по умолчанию")
        redis_host = 'redis-ner'
    redis_port = os.getenv('REDIS_PORT')
    if redis_port is None or redis_port == '':
        print("Значение переменной REDIS_PORT "
              "не установлено, используется значение по умолчанию")
        redis_port = '6379'
    faq_embeddings = os.getenv('FAQ_EMBEDDINGS')
    if faq_embeddings is None or faq_embeddings == '':
        print("Значение переменной FAQ_EMBEDDINGS "
              "не установлено, используется значение по умолчанию")
        faq_embeddings = './model/faq.xlsx'
    rabbit_url = os.getenv('AMQP_SERVER_URL')
    if rabbit_url is None or rabbit_url == '':
        print("Значение переменной AMQP_SERVER_URL "
              "не установлено, используется значение по умолчанию")
        rabbit_url = "amqp://guest:guest@message-broker:5672/"
    queue_in = os.getenv('QUEUE_NAME_IN')
    if queue_in is None or queue_in == '':
        print("Значение переменной QUEUE_NAME_IN "
              "не установлено, используется значение по умолчанию")
        queue_in = 'in'
    queue_out = os.getenv('QUEUE_NAME_OUT')
    if queue_out is None or queue_out == '':
        print("Значение переменной QUEUE_NAME_OUT "
              "не установлено, используется значение по умолчанию")
        queue_out = 'out'


    def _usage_gpu(gpu_check):
        """Подфункция для загрузки поддержки GPU
        gpu_check - проверка True/False в файле конфигурации
        Если установлен True, включается поддержка видеокарты"""
        if gpu_check == 'True':
            if torch.cuda.is_available() is False:
                device = torch.device("cpu")
                log_file(f'Не установленна CUDA для pytorch, используется СPU')
                return device
            try:
                device = torch.device("cuda:0")
                log_file('Выполнена загрузка на GPU: OK')
                return device
            except ValueError as gpu_err:
                log_file(f'Не найдена видеокарта: {gpu_err}')
                return device
            except TypeError as gpu_err:
                log_file(f'Не установленна CUDA для pytorch: {gpu_err}')
                return device
        else:
            log_file('Выполнена загрузка на CPU: OK')
            device = torch.device("cuda:0")
            return device

    device = _usage_gpu(gpu_status)

    return {'Output_Errors': output_file,
            'QA_model': QA_model_pkl,
            'Redis_Host': redis_host,
            'Redis_Port': redis_port,
            'Save_XL_File': save_file,
            'Embeddings': faq_embeddings,
            'Rabbit_url': rabbit_url,
            'Queue_in': queue_in,
            'Queue_out': queue_out,
            'batch_size': int(batch_size),
            'device' : device,
            }


def parse_answer(work_dump, dump_id, config):
    """Функция редактирования результатов
       работы чат бота """

    if config['Save_XL_File'] == '1':
        if not os.path.exists(config['Output_Errors']):
            os.makedirs(config['Output_Errors'])
        result_xl = pd.DataFrame(work_dump)
        writer = pd.ExcelWriter(f"{config['Output_Errors']}/'{datetime.datetime.now()}'.xlsx",
                                engine='xlsxwriter')
        result_xl.to_excel(writer, index=False)
        writer.save()

    message = []
    for strings in work_dump:

        parse_str = {"id": int(dump_id),
                     "error": int(strings['Error']),
                     "Question": str(strings['Question']),
                     "Answer": str(strings['Answer']),
                     "Score": str(strings['Score']),
                     "OperatorFlag": int(strings["OperatorFlag"])
                     }
        message.append(parse_str)

    return message


def QA_bot_module (nn_models, data, redis_connect, config, rabbit_connect, embeddings, dataset):
    """Функция для  генерации бота - сущностей
    nn_models- модели конфигураций spacy
    data - входные данные
    redis_connect - подключение к БД redis
    config - настройки параметров из .env
    rabbit_connect - подключение к rabbitMQ"""

    log_file('Сообщение забрал из очереди')
    
    session = data['sessionId']
    string_count = data['data']
    tokenizer_QA = nn_models[0]
    model_QA = nn_models[1]
    dump_id = 0

    dataframe = []
    try:
        for i in string_count:
            dump_id = i["id"]
            token_text = scripts.QA.find_similar_answers(i["string"], dataset, tokenizer_QA, model_QA, embeddings, top_n=1)

            #result_token = {**token_text}
            dataframe.append(token_text)

    except KeyError as err:
            log_file(f'Ошибка в строке сообщения {err}')
    except TypeError as err:
            log_file(f'Ошибка при отв Проверьте настройки {err}')
          #  continue
    rabbit_settings = [rabbit_connect, config['Queue_out']]

    while True:
        send_mes = send_result(session,
                               parse_answer(dataframe, dump_id, config),
                               rabbit_settings)
        print(send_mes)
        if send_mes == 0:
            log_file('Сообщение отправил в очередь ')
            break
        else:
            time.sleep(2)
            log_file(f'Ошибка при отправке сообщения, переподключаюсь к rabbitmq : {send_mes}')
            connect = pika.BlockingConnection(pika.URLParameters(config['Rabbit_url']))
            rabbit_settings = [connect, config['Queue_out']]
            continue


def redis_init(host, port):
    """ Функция для  для инициализации
        подключения и базы данных"""

    redis_connect = redis_connection(host, port)
    if redis_connect:
        log_file(f'Подключение к Redis - host : {host}, port : {port} ')
    dict_download = import_from_redis(redis_connect)
    if dict_download == 0:
        pass
    else:
        log_file(f'Ошибка подключения к Redis - {dict_download}')
        return 0
    return redis_connect


def get_embeddings_from_dataset(dataset, tokenizer, model, max_length):
    """ Функция для создания эмбендингов
        dataset - excel таблица
        tokenizer - токенизатор модели чат бота
        model - НС чат бота"""
    embeddings = []
    log_file('Создание эмбедингов ')
    for q in dataset['Вопрос']:
        # Tokenize input sequence
        encoded_q = tokenizer(q, return_tensors='pt', truncation=True, max_length=max_length)
        encoded_q = {key: value for key, value in encoded_q.items()}
        with torch.no_grad():
            # Forward pass through the model
            q_embedding = model(**encoded_q).pooler_output
        embeddings.append(q_embedding)
    log_file('Создание эмбедингов : ok' )
    return embeddings
# Getting embeddings from the dataset

def nn_models_downloads(config):
    """ Функция для  для инициализации
        и загрузки в память моделей нейросетей """
    with open(config['QA_model'], "rb") as f:
        loaded_data = pickle.load(f)

    # Получение токенизатора и модели из загруженных данных
    tokenizer_model = loaded_data['tokenizer']
    detect_model = loaded_data['model']
    log_file('Загружены модели : OK ')

    return [tokenizer_model, detect_model]

def main():
    with open("log.txt", 'w', encoding='utf-8') as file:
        start_time = datetime.datetime.now().strftime('%H:%M:%S')
        file.write(f'{start_time} Программа запущена \n')
        print(f'{start_time} Программа запущена ')

    config = config_init()

    log_file('Все переменные окружения прочитаны')
    dataset = pd.read_excel(config['Embeddings'])
    nn_models = nn_models_downloads(config)
    embeddings = get_embeddings_from_dataset(dataset, nn_models[0], nn_models[1], 256)
    redis_connect = None
    rabbit_err_flag = 0

    while True:
        try:
            while not redis_connect:
                redis_connect = redis_init(config['Redis_Host'], config['Redis_Port'])

            connection = pika.BlockingConnection(pika.URLParameters(config['Rabbit_url']))
            rabbit_err_flag = 0
            channel = connection.channel()
            channel.queue_declare(queue=config['Queue_in'], passive=False, durable=True)
            log_file(f"Подключение к RabbitMQ - ok: queue_name = {config['Queue_in']} "
                     f"and url = {config['Rabbit_url']}")

            def callback(ch, method, properties, body):
                db_string = json.loads(body)
                if db_string:
                    QA_bot_module(nn_models, db_string, redis_connect,
                                     config, connection, embeddings, dataset)

            channel.basic_consume(queue=config['Queue_in'],
                                  on_message_callback=callback,
                                  auto_ack=True)
            channel.start_consuming()

        except redis.ConnectionError as err:
            log_file(f'Ошибка подключения к Redis (проверьте настройки) : {err}')
            time.sleep(2)
        except TypeError as err:
            log_file('Hе установлены необходимые переменные окружения')
            time.sleep(2)
        except Exception as err:
            if rabbit_err_flag == 0:
                log_file(f'Ошибка подключения к Rabbitmq - {err}')
                rabbit_err_flag = 1
            time.sleep(2)


if __name__ == "__main__":
    main()
