import os
from flask import Flask, request

os.makedirs('./upload', exist_ok=True)

app = Flask(__name__)

@app.route('/', methods=['GET', 'POST'])
def index():
    if request.method == 'POST':
        print("received")
        print(request.files)
        for _, data in request.files.items():
            print('filename:', data.filename)
            if data.filename:
                data.save(os.path.join('./upload', data.filename))
        return "OK!"

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=8080)