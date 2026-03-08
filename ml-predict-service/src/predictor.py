import joblib
import torch
import numpy as np

class AnomalyPredictor:
    def __init__(self, if_path, custom_path):
        # Classical static methon load
        self.if_model = joblib.load(if_path)
        
        # Deep Learning model load
        self.custom_model = torch.load(custom_path)
        self.custom_model.eval()

    def predict(self, value):
        # Data preparation (reshape for sklearn)
        data = np.array([[value]])
        
        # Isolation Forest: returns -1 for anomalies
        if_res = self.if_model.predict(data)
        
        # Custom Model: inference through tensors
        with torch.no_grad():
            input_tensor = torch.FloatTensor(data)
            output = self.custom_model(input_tensor)
            # Calculate reconstruction error (Loss)
            loss = torch.mean((output - input_tensor)**2).item()
        
        return {
            "if_anomaly": bool(if_res[0] == -1),
            "custom_loss": loss,
            "is_degradation": loss > 0.05 # Your threshold for the neural network
        }