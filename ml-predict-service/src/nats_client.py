import asyncio
import json
import nats
from nats.js.errors import TimeoutError

class NATSClient:
    def __init__(self, server_url, stream_name, subject):
        self.server_url = server_url
        self.stream_name = stream_name
        self.subject = subject
        self.nc = None
        self.js = None

    async def connect(self):
        # Connect to NATS
        self.nc = await nats.connect(self.server_url)
        self.js = self.nc.jetstream()
        print(f"Connected to NATS at {self.server_url}")

    async def subscribe(self, durable_name):
        # Subscribe to the metrics stream.
        # durable_name ensures we continue from the same place on restart.
        return await self.js.subscribe(
            self.subject, 
            stream=self.stream_name,
            durable=durable_name,
            manual_ack=True # Critical for idempotency
        )

    async def publish_prediction(self, prediction_event):
        # Publish the result to a new topic for Read Service
        await self.js.publish(
            "degradation.predicted", 
            json.dumps(prediction_event).encode()
        )

    async def close(self):
        if self.nc:
            await self.nc.close()