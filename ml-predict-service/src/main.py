import asyncio
import os
import json
import signal
from nats_client import NATSClient
from predictor import AnomalyPredictor

async def run():
    """Main execution loop for the ML Predict Service."""
    
    # Configuration from environment variables
    NATS_URL = os.getenv("NATS_URL", "nats://nats:4222")
    IF_MODEL_PATH = "models/isolation_forest.pkl"
    CUSTOM_MODEL_PATH = "models/custom_model.pth"

    # Component initialization
    try:
        predictor = AnomalyPredictor(IF_MODEL_PATH, CUSTOM_MODEL_PATH)
    except FileNotFoundError as e:
        print(f"CRITICAL: Model files not found. Ensure models are placed in {IF_MODEL_PATH} and {CUSTOM_MODEL_PATH}")
        print(f"Error details: {e}")
        return
    except Exception as e:
        print(f"CRITICAL: Unexpected error during model initialization: {e}")
        return

    client = NATSClient(NATS_URL, "METRICS_RECORDED", "metrics.recorded")
    
    await client.connect()
    
    # Create or resume 'ml-analyzer' durable consumer
    sub = await client.subscribe("ml-analyzer")

    print("INFO: ML Engine is listening for metrics...")

    try:
        async for msg in sub.messages:
            try:
                # 1. Decode incoming metric event
                payload = json.loads(msg.data.decode())
                metric_value = payload.get("value")
                metric_name = payload.get("metric_name")

                if metric_value is None or not isinstance(metric_value, (int, float)):
                    print(f"WARN: Skipping invalid payload (missing or invalid 'value'): {payload}")
                    await msg.ack()
                    continue

                # 2. Run dual-model inference
                prediction = predictor.predict(metric_value)

                # 3. If any model detects anomaly/degradation, publish event
                if prediction["if_anomaly"] or prediction["is_degradation"]:
                    event = {
                        "metric_name": metric_name,
                        "source": "ml_predict_service",
                        "results": prediction,
                        "timestamp": payload.get("timestamp")
                    }
                    await client.publish_prediction(event)
                    print(f"DEBUG: Degradation predicted for {metric_name}")

                # 4. Explicitly acknowledge message only after successful processing
                await msg.ack()

            except json.JSONDecodeError as e:
                print(f"ERROR: Unparseable JSON, discarding message: {e}")
                await msg.ack()  # Ack poison messages to avoid infinite loops
            except Exception as e:
                print(f"ERROR: Failed to process message: {e}")
                # Negative acknowledgement allows the message to be retried for transient errors
                await msg.nak()

    except asyncio.CancelledError:
        print("INFO: Service cancellation requested")
    finally:
        await client.close()
        print("INFO: NATS connection closed. Clean exit.")

if __name__ == "__main__":
    # Standard Unix signal handling for graceful shutdown
    loop = asyncio.get_event_loop()
    main_task = loop.create_task(run())
    
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, main_task.cancel)

    try:
        loop.run_until_complete(main_task)
    except Exception:
        pass