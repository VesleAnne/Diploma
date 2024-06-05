# -*- coding: utf-8 -*-


import torch
from sklearn.metrics.pairwise import cosine_similarity
import pandas as pd

import pickle

# Load data
max_length = 256


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
        answ = dataset['Схема ответа'][idx]
        web = dataset['Ссылка на wiki'][idx]
        if (web is None):
            web = ''
        score_array = (dataset['Схема ответа'][idx], similarities[idx])

    # Returning the most similar answers and their corresponding proximity probabilities
    for score in score_array:
        score_convert = score[0]


    output_bot_answ = {
                            'Error': False,
                            "Question": question,
                            "Answer": answ + "\n " + str(web),
                            "Score": score_convert[0],
                            "OperatorFlag": 0
                     }
    return output_bot_answ





