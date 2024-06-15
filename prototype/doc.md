# Calculating the outgoing amount by a given incoming amount

## outgoing -> incoming
Let
- $r_o$ the outbound fee rate
- $b_o$ the outbound base fee
- $r_i$ the inbound fee rate
- $b_i$ the inbound base fee

We express the incoming amount $g(x)$ as a function of the outgoing amount $x$:

$$
g(x)=\max(f(x), x)
$$

with

$$
f(x)=(x+r_ox+b_o)+(x+r_ox+b_o)\cdot r_i +b_i
$$

The function $f$ calculates the incoming amount ignoring the floor by the
outgoing amount. It becomes $f(x)=m\cdot x +n$ if we set 

$$
m:=1+r_o+r_i+r_o\cdot r_i=(1+r_o)\cdot (1+ r_i)
$$

$$
n:=b_o+b_i+b_o\cdot r_i
$$

Moreover $m>0$ is equivalent to $r_i>-1$ which makes sense in practice, as 
nobody will give 100% or more inbound discount.

## incoming -> outgoing
Now, given an incoming amount $y$, we are looking for $x$ which satisfies $y=\max(f(x),x)$.

- 1st case $f(x)\leq x$: In this case we can set $x=y$ and this leads to $f(y)\leq y$.

- 2nd case $f(x)>x$: In this case $y=f(x)$ and hence $x=\frac{y-n}{m}$. Because
of $m>0$ we also get

$$
f(\frac{y-n}{m})>\frac{y-n}{m}\Leftrightarrow y>\frac{y-n}{m}\Leftrightarrow y\cdot m +n>y\Leftrightarrow f(y)>y
$$

Because of case one $f(y)>y$ implies $f(x)>x$ and because of case two $f(y)\leq y$
implies $f(x)\leq x$ if $m>0$. That leads us to a situation that we can determine 
the relevant case by comparing $f(y)$ and $y$. Remark that this conclusion is only
possible because of $m>0$. If not, the inequality in case two would change.
Then it wouldn't be possible to determine the case in this way.