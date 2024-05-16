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

    # Returning the most similar answers and their corresponding proximity probabilities
    similar_answers = [(dataset['Схема ответа'][idx], similarities[idx]) for idx in top_indices]
    for score in similar_answers[0][1][0].astype(float):
        score_convert = float(score)

    output_bot_answ = {
                            'Error': False,
                            "Question": question,
                            "Answer": similar_answers[0][0],
                            "Score": score_convert,
                            "OperatorFlag": False
                     }
    return output_bot_answ





