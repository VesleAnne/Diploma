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

'''
    try:
        with open('FNdict', "r", encoding='utf-8') as dictionary:
            lines_file = dictionary.readlines()
            if redis_connect.hlen('FN') != len(lines_file):
                fnl = 0
                for line in lines_file:
                    redis_connect.hset('FN', line, fnl)
                    fnl += 1
        with open('LNdict', "r", encoding='utf-8') as dictionary:
            lines_file = dictionary.readlines()
            if redis_connect.hlen('LN') != len(lines_file):
                lnl = 0
                for line in lines_file:
                    redis_connect.hset('LN', line, lnl)
                    lnl += 1

        return err
    except Exception as err:
    
        return err
    '''



