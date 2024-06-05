# -*- coding: utf-8 -*-


import torch
from sklearn.metrics.pairwise import cosine_similarity
import pandas as pd

import pickle

# Load data
max_length = 256

def get_embeddings_from_dataset(dataset, tokenizer, model, max_length):
    embeddings = []
    for q in dataset['Вопрос']:
        # Tokenize input sequence
        encoded_q = tokenizer(q, return_tensors='pt', truncation=True, max_length=max_length)
        encoded_q = {key: value for key, value in encoded_q.items()}
        with torch.no_grad():
            # Forward pass through the model
            q_embedding = model(**encoded_q).pooler_output
        embeddings.append(q_embedding)
    return embeddings

def find_similar_answers(question, dataset, tokenizer, model, embeddings, top_n=1):
    # Tokenizing the input question
    encoded_input = tokenizer(question, return_tensors='pt')

    # Getting the question embeddings from the model
    with torch.no_grad():
        question_embedding = model(**encoded_input).pooler_output.cpu()  # Move to CPU
        # Computing the cosine similarity between the question and all questions in the dataset with added random noise
    similarities = []
    for q_embedding in embeddings:
        similarity = cosine_similarity(question_embedding, q_embedding.cpu())  # Move to CPU
        similarities.append(similarity)

    # Getting indexes of the most similar questions
    top_indices = sorted(range(len(similarities)), key=lambda i: similarities[i], reverse=True)[:top_n]
    for idx in top_indices:
        print(idx)
        answ = dataset['Схема ответа'][idx]
        web = dataset['Ссылка на wiki'][idx]
        if (web is None):
            web = ''
        score_array = (dataset['Схема ответа'][idx], similarities[idx])

    # Returning the most similar answers and their corresponding proximity probabilities
    for score in score_array:
        score_convert = score[0]
        print(score_convert)



    output_bot_answ = {
                            'error': False,
                            "Question": question,
                            "Answer": answ + "\n " + web,
                            "Score": score_convert[0],
                            "OperatorFlag": 0
                     }

    print(output_bot_answ)
    return output_bot_answ


with open(r'/home/user/ner/Diploma/greendata/model/tokenizer_and_model.pkl', "rb") as f:
    loaded_data = pickle.load(f)

tokenizer_model = loaded_data['tokenizer']
detect_model = loaded_data['model']

dataset = pd.read_excel(r'/home/user/ner/Diploma/greendata/model/faq.xlsx')
embed = get_embeddings_from_dataset(dataset,tokenizer_model,detect_model, 256)

find_similar_answers("Зачем привязываться к юридическому лицу?",dataset,tokenizer_model,detect_model, embed)
