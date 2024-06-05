import redis


def unique_list(string_from_duplicate):
    """Функция исключения дубликатов"""

    dualist = list(set(string_from_duplicate))
    return dualist


def redis_connection(host, port):
    """Подключение к БД Redis"""

    connect = redis.Redis(host, int(port))
    return connect


def import_from_redis(redis_connect):
    """Функция для загрузки ответов в БД Redis"""
    err = 0
    return err


