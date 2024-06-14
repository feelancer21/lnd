import random
from types import SimpleNamespace

UNIT_M = 1_000_000

class FeeStructure():
    def __init__(self,
        fee_rate_out: int,
        base_out: int,
        fee_rate_in: int,
        base_in: int) -> None:
        
        self.m = (
            1 + fee_rate_out / UNIT_M * (1 + fee_rate_in / UNIT_M) + 
            fee_rate_in / UNIT_M
        )

        self.n = base_out * (1 + fee_rate_in / UNIT_M) + base_in
    
    # Helper function. Returns the inbound amount ignoring the floor by
    # the outbound.
    def f(self, x: float):
        return self.m * x + self.n
    
    # Calculates the inbound amount by a given outbound amount, which is
    # the calculation when building a route starting from the receiver.
    def inbound_from_outbound(self, amount: int) -> float:
        return max(self.f(amount), amount)
    
    # Our derived inverse for calculating the outbound by a given inbound.
    def outbound_from_inbound(self, amount: float) -> float:
        if self.f(amount) > amount:
            return (amount - self.n) / self.m
        return amount

class TestCase(SimpleNamespace):
    fee_rate_out: int
    base_out: int
    fee_rate_in: int
    base_in: int
    fee: FeeStructure
    amt_out: int
    amt_in: float
    amt_inv: float

if __name__ == "__main__":
    random.seed(21_000_000)
    # Running over with 1M different fee structures with 3 different amount
    for _ in range(1_000_000):
        t = TestCase
        t.fee_rate_out=random.randint(0, 10000)
        t.base_out=random.randint(0, 2000)
        t.fee_rate_in=random.randint(-10000, 10000)
        t.base_in=random.randint(-2000, 2000)

        t.fee = FeeStructure(
            t.fee_rate_out,
            t.base_out,
            t.fee_rate_in,
            t.base_in
        )

        # Generating random amounts in msats.
        # Test for a amount below 1_000 msat.
        t.amt_out = random.randint(0,1_000)
        t.amt_in = t.fee.inbound_from_outbound(t.amt_out)
        t.amt_inv = t.fee.outbound_from_inbound(t.amt_in)
        if round(t.amt_inv,0) != t.amt_out:
            print(f'test failed: {t.__dict__}')

        # Test for a amount between 1 sat and 1M sat.
        t.amt_out = random.randint(1_000, 1_000_000_000)
        t.amt_in = t.fee.inbound_from_outbound(t.amt_out)
        t.amt_inv = t.fee.outbound_from_inbound(t.amt_in)
        if round(t.amt_inv,0) != t.amt_out:
            print(f'test failed: {t.__dict__}')

        # Test for a amount between 1M sat and 1BTC.
        t.amt_out = random.randint(1_000_000_000, 100_000_000_000)
        t.amt_in = t.fee.inbound_from_outbound(t.amt_out)
        t.amt_inv = t.fee.outbound_from_inbound(t.amt_in)
        if round(t.amt_inv,0) != t.amt_out:
            print(f'test failed: {t.__dict__}')
