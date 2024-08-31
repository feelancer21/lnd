Hello Matt, 
would like to hijack the issue, as I also have methodological concerns about the
LinearFeeFunction, which I have illustrated below with another alternative. 

I'd be interested in feedback from those reading along on the alternative.
If there are no objections, I would like to start a first draft in the coming weeks.

Btw: Regardless of the methodology, there still seems to be an issue with calling
the fee function. Otherwise it is not possible to explain why this pleb ended up
at 1000 sat/vb although he was permanently online and onchain fees were low.

https://mempool.space/tx/2a5b92a5be69799f9eb919e54426248c6fcb9c85e44692711761e34b1b1b2068

### Downsides of the LinearFeeFunction

The `LinearFeeFunction` is initialized with a `maxFeeRate`, which is set by 
default to 1000 sat/vbyte, and can be adjusted via configuration down to 
100 sat/vbyte. I see some downsides. If I have misunderstood something, don't
hesitate to correct me.

- When using the default setting, the function becomes very steep depending on 
  CLTV. With a CLTV of 40 blocks, the linear fee function increases by about 
  25 sats/vbyte per block. At 144 blocks, the increase is around 7 sats/vbyte 
  per block.

- The function becomes steeper if it is initialized at a later stage, such as 
  when a node comes back online after an extended downtime. In the extreme 
  case where we need to sweep a pending HTLC that has already exceeded its 
  deadline, we are forced to pay the `maxFeeRate`.

- This also means that two fee functions that were initialized with the same
  absolute deadline but at different times due to different CLTVs have different
  values and steepnesses for the same conf_target.

- Node runners can only control the steepness of the function by lowering the 
  `maxFeeRate` or the budgets via configuration. While they may do so, with 
  a 100 sat/vbyte floor, people risk losing their funds in a high-fee environment 
  because the linear fee function is capped at this value. But people are more
  likely to forget.

### Alternatives

#### `endingFeeRate` depending on on-chain fees

We could make the `endingFeeRate` dynamic by using the current on-chain 
fee rate for a conf_target of 1 or 2. However, this approach risks a sweep 
not getting confirmed if on-chain fee rates increase very slowly.

#### Model with a limited exponential fee function

We define a function $g(x)$, which depends on the current conf_target $x$. 
The actual fee function is `min(maxFeeRate; g(x))`.

For the definition of $g$, we need the following parameters:
- $\lambda \in [0;1]$
- $m \geq 0$
- The current on-chain fee $f(x)$ at the time the function is called

Now we can define:

$$
g(x):=(m\lambda^{x-1} + 1)\cdot f(x)
$$

This function has the following properties:
- $g(x) \geq f(x)$, meaning it always returns values at or above the current 
  on-chain fees.
- $g(x) \leq (m+1)\cdot f(x)$ and $g(1)=(1+m)\cdot f(1)$, which means our fee 
  function has an upper limit with a safety buffer capped at $m\cdot f(x)$. 
  This buffer is dynamic and scales with the current on-chain fees.
- $\lambda$ allows us to control the speed at which we approach the maximum of 
  $(m+1)\cdot f(x)$.

Let's examine how the factors $m\lambda^{x-1} + 1$ behave for different 
scenarios of conf_targets $x$ and various $\lambda$ values. We assume $m=1$, 
meaning we pay a maximum of twice the current on-chain fees.

|       | **0.9** | **0.95** | **0.975** | **0.99** |
|-------|---------|----------|-----------|----------|
| **1** | 2       | 2        | 2         | 2        |
| **6** | 1.59    | 1.77     | 1.88      | 1.95     |
| **40**| 1.02    | 1.14     | 1.37      | 1.68     |
| **144**| 1      | 1        | 1.03      | 1.24     |

With $\lambda=0.975$, we see a surcharge of 37% on top of the on-chain fees 
40 blocks before the deadline, making it less likely that confirmation will 
not occur before this time. If $m=2$, the surcharge is doubled.

Overall, the likelihood of not getting a confirmation with the model is very
unlikely. This is actually only possible if the fees increase faster from block
to block than the risk buffer at the time.

I would greatly appreciate any suggestions or thoughts on this approach and 
am willing to prepare a draft in the coming weeks.
