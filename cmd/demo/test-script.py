from flask import Flask, Response
import threading
import time
import random
import requests
import json

app = Flask(__name__)
@app.after_request
def add_cors_headers(response):
    response.headers['Access-Control-Allow-Origin'] = '*'
    response.headers['Access-Control-Allow-Methods'] = 'GET,POST,OPTIONS'
    response.headers['Access-Control-Allow-Headers'] = 'Content-Type,Authorization'
    return response


@app.route('/', methods=['GET'])
def some_route():
    return Response("he the goat", mimetype='text/plain')


def wait_for_server(url, timeout=10):
    deadline = time.time() + timeout
    random_data = {"foo": random.randint(1, 100), "bar": random.choice(["baz", "qux"])}
    while time.time() < deadline:
        try:
            requests.post(url, data=json.dumps(random_data), headers={"Content-Type": "application/json"}, timeout=1)
            return
        except requests.exceptions.ConnectionError:
            time.sleep(0.1)
    raise RuntimeError(f"Server at {url} did not become ready")


def periodic_request():
    url = "http://localhost:8080/some"
    while True:
        wait_time = random.uniform(0, 5)
        time.sleep(wait_time)
        try:
            payload = {
                "foo": random.randint(1, 100),
                "bar": random.choice(["baz", "qux"])
            }
            response = requests.post(
                url,
                data=json.dumps(payload),
                headers={"Content-Type": "application/json"},
                timeout=5
            )
            print(f"Hit {url}, sent: {payload}, got: {response.text}")
        except Exception as e:
            print(f"Request failed: {e}")


if __name__ == "__main__":
    server_thread = threading.Thread(
        target=lambda: app.run(host='localhost', port=8000),
        daemon=True
    )
    server_thread.start()

    url = "http://localhost:8080/some"
    wait_for_server(url)
    periodic_request()
