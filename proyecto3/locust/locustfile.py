import random
from datetime import datetime, timezone
from locust import HttpUser, task, between

COUNTRIES = ["USA", "RUS", "CHN", "ESP", "GTM"]

class WarReportUser(HttpUser):
    wait_time = between(0.1, 0.5)

    @task
    def send_report(self):
        payload = {
            "country": random.choice(COUNTRIES),
            "warplanes_in_air": random.randint(0, 50),
            "warships_in_water": random.randint(0, 30),
            "timestamp": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        }
        self.client.post("/grpc-202300559", json=payload)
